package gcalbot

import (
	"bytes"
	"context"
	"crypto/hmac"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/managed-bots/base"
	"golang.org/x/oauth2"
)

type HTTPSrv struct {
	*base.OAuthHTTPSrv

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
		db:                db,
		handler:           handler,
		reminderScheduler: reminderScheduler,
	}
	h.OAuthHTTPSrv = base.NewOAuthHTTPSrv(stats, kbc, debugConfig, oauthConfig, h.db, h.handler.HandleAuth,
		"gcalbot", base.Images["logo"], "/gcalbot")
	http.HandleFunc("/gcalbot", h.configHandler)
	http.HandleFunc("/gcalbot/home", h.homeHandler)
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

func (h *HTTPSrv) configHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	defer func() {
		if err != nil {
			h.Errorf("error in configHandler: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("something went wrong :("))
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
	keybaseConvName := keybaseConv.Channel.Name

	accountNickname := r.Form.Get("account")
	calendarID := r.Form.Get("calendar")

	previousAccountNickname := r.Form.Get("previous_account")
	previousCalendarID := r.Form.Get("previous_calendar")

	reminderInput := r.Form.Get("reminder")
	inviteInput := r.Form.Get("invite")

	accounts, err := h.db.GetAccountNicknameListForUsername(keybaseUsername)
	if err != nil {
		return
	}

	page := ConfigPage{
		Title:           "gcalbot | config",
		KeybaseConvID:   keybaseConvID,
		KeybaseConvName: keybaseConvName,
		Account:         accountNickname,
		Accounts:        accounts,
		Reminders:       reminders,
	}

	if accountNickname == "" {
		h.servePage(w, "config", page)
		return
	}

	accountID := GetAccountID(keybaseUsername, accountNickname)

	token, err := h.db.GetToken(accountID)
	if err != nil || token == nil {
		return
	}

	client := h.handler.config.Client(context.Background(), token)
	srv, err := calendar.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return
	}

	calendarList, err := srv.CalendarList.List().Do()
	if err != nil {
		return
	}
	page.Calendars = calendarList.Items

	if accountNickname == previousAccountNickname {
		// if the account hasn't changed, display the selected calendar (otherwise clear selected calendar)
		page.CalendarID = calendarID
	} else {
		h.servePage(w, "config", page)
		return
	}

	var subscriptions []*Subscription
	subscriptions, err = h.db.GetSubscriptions(accountID, calendarID, keybaseConvID)
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

	if calendarID == previousCalendarID {
		// if the calendar hasn't changed, update the settings
		inviteSubscription := Subscription{
			AccountID:     accountID,
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
			_, err = h.handler.removeSubscription(srv, inviteSubscription)
			if err != nil {
				return
			}
		} else if !page.Invite && invite {
			// create invite subscription
			_, err = h.handler.createSubscription(srv, inviteSubscription)
			if err != nil {
				return
			}
		}
		page.Invite = invite

		if page.Reminder != "" {
			// remove old reminder subscription
			var oldMinutesBefore int
			oldMinutesBefore, err = strconv.Atoi(page.Reminder)
			if err != nil {
				return
			}

			_, err = h.handler.removeSubscription(srv, Subscription{
				AccountID:      accountID,
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
			var newMinutesBefore int
			newMinutesBefore, err = strconv.Atoi(reminderInput)
			if err != nil {
				return
			}

			_, err = h.handler.createSubscription(srv, Subscription{
				AccountID:      accountID,
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

func (h *HTTPSrv) screenshotHandler(w http.ResponseWriter, r *http.Request) {
	dat, _ := base64.StdEncoding.DecodeString(screenshot)
	if _, err := io.Copy(w, bytes.NewBuffer(dat)); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (h *HTTPSrv) showLoginInstructions(w http.ResponseWriter) {
	w.WriteHeader(http.StatusForbidden)
	h.servePage(w, "login", LoginPage{Title: "gcalbot | login"})
}

func (h *HTTPSrv) authUser(w http.ResponseWriter, r *http.Request) (keybaseUsername string, keybaseConvID chat1.ConvIDStr, ok bool) {
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
