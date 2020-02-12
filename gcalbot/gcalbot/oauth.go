package gcalbot

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/keybase/managed-bots/base"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"golang.org/x/oauth2"
)

func (h *HTTPSrv) oauthHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	defer func() {
		if err != nil {
			h.Errorf("oauthHandler: %v", err)
			h.showOAuthError(w)
		}
	}()

	if r.URL == nil {
		err = fmt.Errorf("r.URL == nil")
		return
	}

	query := r.URL.Query()
	state := query.Get("state")

	req, err := h.db.GetState(state)
	if err != nil {
		err = fmt.Errorf("could not get state %q: %v", state, err)
		return
	} else if req == nil {
		// no state is found
		h.showOAuthError(w)
		return
	}

	if req.IsComplete {
		_, err = w.Write(base.MakeOAuthHTML("gcalbot", "success",
			`<div class="success"> Success! </div>
		<div>You can now close this page and return to the Keybase app.</div>`,
			"/gcalbot/image/logo"))
		if err != nil {
			h.Errorf("oauthHandler: unable to write: %v", err)
		}
		return
	}

	code := query.Get("code")
	if code == "" {
		// no code is provided
		h.showOAuthError(w)
		return
	}
	token, err := h.oauth.Exchange(context.TODO(), code)
	if err != nil {
		return
	}

	err = h.db.InsertAccount(Account{
		KeybaseUsername: req.KeybaseUsername,
		AccountNickname: req.AccountNickname,
		Token:           *token,
	})
	if err != nil {
		return
	}
	if err = h.db.CompleteState(state); err != nil {
		return
	}

	loginToken := h.handler.LoginToken(req.KeybaseUsername)

	redirectQuery := url.Values{}
	redirectQuery.Add("token", loginToken)
	redirectQuery.Add("username", req.KeybaseUsername)
	redirectQuery.Add("account", req.AccountNickname)
	redirectQuery.Add("conv_id", string(req.KeybaseConvID))
	path := fmt.Sprintf("/gcalbot?%s", redirectQuery.Encode())

	http.Redirect(w, r, path, http.StatusSeeOther)
}

func (h *HTTPSrv) showOAuthError(w http.ResponseWriter) {
	if _, err := w.Write(base.MakeOAuthHTML("gcalbot", "error",
		"Unable to complete request, please try running the bot command again!",
		"/gcalbot/image/logo")); err != nil {
		h.Errorf("oauthHandler: unable to write: %v", err)
	}
}

func (h *Handler) requestOAuth(msg chat1.MsgSummary, accountNickname string) error {
	state, err := base.MakeRequestID()
	if err != nil {
		return err
	}

	err = h.db.PutState(state, OAuthRequest{
		KeybaseUsername: msg.Sender.Username,
		AccountNickname: accountNickname,
		KeybaseConvID:   msg.ConvID,
	})
	if err != nil {
		return err
	}

	oauthOpts := []oauth2.AuthCodeOption{oauth2.ApprovalForce, oauth2.AccessTypeOffline}
	authURL := h.oauth.AuthCodeURL(state, oauthOpts...)
	_, err = h.kbc.SendMessageByTlfName(msg.Sender.Username,
		"Visit %s to connect a Google account as '%s'.", authURL, accountNickname)
	if err != nil {
		return err
	}

	// If we are in a 1-1 conv directly or as a bot user with the sender, skip this message.
	if !base.IsDirectPrivateMessage(h.kbc.GetUsername(), msg) {
		h.ChatEcho(msg.ConvID, "OK! I've sent a message to @%s to authorize me.", msg.Sender.Username)
	}

	return nil
}
