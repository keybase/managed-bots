package gcalbot

import (
	"bytes"
	"crypto/hmac"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/managed-bots/base"
	"golang.org/x/oauth2"
)

type HTTPSrv struct {
	*base.HTTPSrv

	kbc     *kbchat.API
	oauth   *oauth2.Config
	db      *DB
	handler *Handler

	reminderScheduler ReminderScheduler
}

func NewHTTPSrv(
	stats *base.StatsRegistry,
	kbc *kbchat.API,
	debugConfig *base.ChatDebugOutputConfig,
	db *DB,
	oauthConfig *oauth2.Config,
	reminderScheduler ReminderScheduler,
	handler *Handler,
) *HTTPSrv {
	h := &HTTPSrv{
		kbc:               kbc,
		oauth:             oauthConfig,
		db:                db,
		handler:           handler,
		reminderScheduler: reminderScheduler,
	}
	h.HTTPSrv = base.NewHTTPSrv(stats, debugConfig)
	http.HandleFunc("/gcalbot", h.configHandler)
	http.HandleFunc("/gcalbot/healthcheck", h.healthCheckHandler)
	http.HandleFunc("/gcalbot/home", h.homeHandler)
	http.HandleFunc("/gcalbot/oauth", h.oauthHandler)
	http.HandleFunc("/gcalbot/image/logo", h.logoHandler)
	http.HandleFunc("/gcalbot/image/screenshot", h.screenshotHandler)
	http.HandleFunc("/gcalbot/events/webhook", h.handleEventUpdateWebhook)
	return h
}

var reminders = []ReminderType{
	{"0", "At time of event"},
	{"1", "1 minute before"},
	{"5", "5 minutes before"},
	{"10", "10 minutes before"},
	{"15", "15 minutes before"},
	{"30", "30 minutes before"},
	{"60", "60 minutes before"},
}

func (h *HTTPSrv) healthCheckHandler(w http.ResponseWriter, r *http.Request) {}

func (h *HTTPSrv) configHandler(w http.ResponseWriter, r *http.Request) {
	h.Stats.Count("config")
	var err error
	defer func() {
		if err != nil {
			h.Errorf("error in configHandler: %s", err)
			h.showConfigError(w)
		}
	}()

	err = r.ParseForm()
	if err != nil {
		return
	}

	keybaseUsername, keybaseConvID, ok := h.authUser(w, r)
	if !ok {
		h.showLoginInstructions(w)
		return
	}

	keybaseConv, err := h.handler.kbc.GetConversation(keybaseConvID)
	if err != nil {
		return
	}

	isAdmin, err := base.IsAdmin(h.kbc, keybaseUsername, keybaseConv.Channel)
	if err != nil {
		return
	} else if !isAdmin {
		// should only be able to configure notifications if isAdmin
		h.showConfigError(w)
		return
	}

	isPrivate := base.IsDirectPrivateMessage(h.kbc.GetUsername(), keybaseUsername, keybaseConv.Channel)

	accountNickname := r.Form.Get("account")
	calendarID := r.Form.Get("calendar")

	previousAccountNickname := r.Form.Get("previous_account")
	previousCalendarID := r.Form.Get("previous_calendar")

	reminderInput := r.Form.Get("reminder")
	inviteInput := r.Form.Get("invite")

	accounts, err := h.db.GetAccountListForUsername(keybaseUsername)
	if err != nil {
		return
	}

	if len(accounts) == 0 {
		h.servePage(w, "account help", AccountHelpPage{
			Title: "gcalbot | config",
		})
		return
	}

	page := ConfigPage{
		Title:         "gcalbot | config",
		ConvID:        keybaseConvID,
		ConvHelpText:  GetConvHelpText(keybaseConv.Channel, false),
		ConvIsPrivate: isPrivate,
		Account:       accountNickname,
		Accounts:      accounts,
		Reminders:     reminders,
	}

	if accountNickname == "" {
		h.servePage(w, "config", page)
		return
	}

	var selectedAccount *Account
	for _, account := range accounts {
		if account.AccountNickname == accountNickname {
			selectedAccount = account
		}
	}
	if selectedAccount == nil {
		h.showConfigError(w)
		return
	}

	srv, err := GetCalendarService(selectedAccount, h.oauth)
	if err != nil {
		return
	}

	calendarList, err := srv.CalendarList.List().Do()
	if err != nil {
		return
	}
	page.Calendars = calendarList.Items

	// default to the primary calendar
	if calendarID == "" {
		for _, calendarItem := range calendarList.Items {
			if calendarItem.Primary {
				calendarID = calendarItem.Id
			}
		}
	}
	page.CalendarID = calendarID

	if accountNickname != previousAccountNickname {
		// if the account has changed, clear the previous calendarID
		previousCalendarID = ""
	}

	var subscriptions []*Subscription
	subscriptions, err = h.db.GetSubscriptions(selectedAccount, calendarID, keybaseConvID)
	if err != nil {
		return
	}
	for _, subscription := range subscriptions {
		switch subscription.Type {
		case SubscriptionTypeInvite:
			page.Invite = true
		case SubscriptionTypeReminder:
			page.Reminder = strconv.Itoa(GetMinutesFromDuration(subscription.DurationBefore))
		}
	}

	// if the calendar hasn't changed, update the settings
	if calendarID == previousCalendarID {
		h.Stats.Count("config - update")
		// the conv must be private (direct message) for the user to subscribe to invites
		if isPrivate {
			h.Stats.Count("config - update - direct message")
			inviteSubscription := Subscription{
				CalendarID:    calendarID,
				KeybaseConvID: keybaseConvID,
				Type:          SubscriptionTypeInvite,
			}
			var invite bool
			if inviteInput != "" {
				invite = true
			}

			if page.Invite && !invite {
				// remove invite subscription
				h.Stats.Count("config - update - invite - remove")
				err = h.handler.removeSubscription(selectedAccount, inviteSubscription)
				if err != nil {
					return
				}
			} else if !page.Invite && invite {
				// create invite subscription
				h.Stats.Count("config - update - invite - create")
				_, err = h.handler.createSubscription(selectedAccount, inviteSubscription)
				if err != nil {
					return
				}
			}
			page.Invite = invite
		} else {
			h.Stats.Count("config - update - team")
		}

		if page.Reminder != "" {
			// remove old reminder subscription
			h.Stats.Count("config - update - reminder - remove")
			var oldMinutesBefore int
			oldMinutesBefore, err = strconv.Atoi(page.Reminder)
			if err != nil {
				return
			}

			err = h.handler.removeSubscription(selectedAccount, Subscription{
				CalendarID:     calendarID,
				KeybaseConvID:  keybaseConvID,
				DurationBefore: GetDurationFromMinutes(oldMinutesBefore),
				Type:           SubscriptionTypeReminder,
			})
			if err != nil {
				return
			}
		}
		if reminderInput != "" {
			// create new reminder subscription
			h.Stats.Count("config - update - reminder - create")
			var newMinutesBefore int
			newMinutesBefore, err = strconv.Atoi(reminderInput)
			if err != nil {
				return
			}

			_, err = h.handler.createSubscription(selectedAccount, Subscription{
				CalendarID:     calendarID,
				KeybaseConvID:  keybaseConvID,
				DurationBefore: GetDurationFromMinutes(newMinutesBefore),
				Type:           SubscriptionTypeReminder,
			})
			if err != nil {
				return
			}
		}
		page.Reminder = reminderInput
	}

	h.servePage(w, "config", page)
}

func (h *HTTPSrv) showConfigError(w http.ResponseWriter) {
	h.Stats.Count("configError")

	w.WriteHeader(http.StatusInternalServerError)
	h.servePage(w, "error", ErrorPage{
		Title: "gcalbot | error",
	})
}

func (h *HTTPSrv) homeHandler(w http.ResponseWriter, r *http.Request) {
	h.Stats.Count("home")
	homePage := `Google Calendar Bot is a <a href="https://keybase.io">Keybase</a> chatbot
	which connects with your Google calendar to notify you of invites, upcoming events and more!
	<div style="padding-top:25px;">
		<img style="width:300px;" src="/gcalbot/image/screenshot">
	</div>
	`
	if _, err := w.Write(base.MakeOAuthHTML("gcalbot", "home", homePage, "/gcalbot/image/logo")); err != nil {
		h.Errorf("homeHandler: unable to write: %v", err)
	}
}

func (h *HTTPSrv) logoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Cache-Control", "max-age=86400")
	dat, _ := base64.StdEncoding.DecodeString(base.Images["logo"])
	if _, err := io.Copy(w, bytes.NewBuffer(dat)); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (h *HTTPSrv) screenshotHandler(w http.ResponseWriter, r *http.Request) {
	dat, _ := base64.StdEncoding.DecodeString(screenshot)
	if _, err := io.Copy(w, bytes.NewBuffer(dat)); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (h *HTTPSrv) showLoginInstructions(w http.ResponseWriter) {
	h.Stats.Count("loginInstructions")
	w.WriteHeader(http.StatusForbidden)
	h.servePage(w, "login", LoginPage{Title: "gcalbot | login"})
}

func (h *HTTPSrv) authUser(w http.ResponseWriter, r *http.Request) (keybaseUsername string, keybaseConvID chat1.ConvIDStr, ok bool) {
	h.Stats.Count("authUser")
	keybaseUsername = r.Form.Get("username")
	token := r.Form.Get("token")
	keybaseConvID = chat1.ConvIDStr(r.Form.Get("conv_id"))

	if keybaseConvID == "" {
		return "", "", false
	}

	if keybaseUsername == "" || token == "" {
		cookie, err := r.Cookie("auth")
		if err != nil {
			h.Debug("error getting cookie: %s", err)
			return "", "", false
		}
		if cookie == nil {
			return "", "", false
		}
		auth := cookie.Value
		toks := strings.Split(auth, ":")
		if len(toks) != 2 {
			h.Debug("malformed auth cookie", auth)
			return "", "", false
		}
		keybaseUsername = toks[0]
		token = toks[1]
	}

	realToken := h.handler.LoginToken(keybaseUsername)
	if !hmac.Equal([]byte(realToken), []byte(token)) {
		h.Debug("invalid auth token")
		return "", "", false
	}
	http.SetCookie(w, &http.Cookie{
		Name:    "auth",
		Value:   fmt.Sprintf("%s:%s", keybaseUsername, token),
		Expires: time.Now().Add(8760 * time.Hour),
	})
	return keybaseUsername, keybaseConvID, true
}
