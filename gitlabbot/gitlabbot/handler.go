package gitlabbot

import (
	"context"
	"fmt"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
)

type Handler struct {
	*base.DebugOutput

	stats      *base.StatsRegistry
	kbc        *kbchat.API
	db         *DB
	httpPrefix string
	secret     string
}

var _ base.Handler = (*Handler)(nil)

func NewHandler(
	stats *base.StatsRegistry,
	kbc *kbchat.API,
	debugConfig *base.ChatDebugOutputConfig,
	db *DB,
	httpPrefix string,
	secret string,
) *Handler {
	return &Handler{
		DebugOutput: base.NewDebugOutput("Handler", debugConfig),
		stats:       stats.SetPrefix("Handler"),
		kbc:         kbc,
		db:          db,
		httpPrefix:  httpPrefix,
		secret:      secret,
	}
}

func (h *Handler) HandleNewConv(_ context.Context, conv chat1.ConvSummary) error {
	welcomeMsg := "Hi! I can notify you whenever something happens on a GitLab repository. To get started, set up a repository by sending `!gitlab subscribe <owner/repo>`"
	return base.HandleNewTeam(h.stats, h.DebugOutput, h.kbc, conv, welcomeMsg)
}

func (h *Handler) HandleAuth(ctx context.Context, msg chat1.MsgSummary, _ string) error {
	return h.HandleCommand(ctx, msg)
}

func (h *Handler) HandleCommand(ctx context.Context, msg chat1.MsgSummary) error {
	if msg.Content.Text == nil {
		return nil
	}

	cmd := strings.ToLower(strings.TrimSpace(msg.Content.Text.Body))
	if !strings.HasPrefix(cmd, "!gitlab") {
		return nil
	}

	switch {
	case strings.HasPrefix(cmd, "!gitlab subscribe"):
		h.stats.Count("subscribe")
		return h.handleSubscribe(ctx, cmd, msg, true)
	case strings.HasPrefix(cmd, "!gitlab unsubscribe"):
		h.stats.Count("unsubscribe")
		return h.handleSubscribe(ctx, cmd, msg, false)
	case strings.HasPrefix(cmd, "!gitlab list"):
		h.stats.Count("list")
		return h.handleListSubscriptions(ctx, msg)
	}
	return nil
}

func (h *Handler) handleSubscribe(ctx context.Context, cmd string, msg chat1.MsgSummary, create bool) (err error) {
	toks, userErr, err := base.SplitTokens(cmd)
	if err != nil {
		return err
	} else if userErr != "" {
		h.ChatEcho(msg.ConvID, "%s", userErr)
		return nil
	}

	args := toks[2:]
	if len(args) < 1 {
		h.ChatEcho(msg.ConvID, "Bad arguments for subscribe: %v", args)
		return nil
	}

	if ok, err := base.IsAtLeastWriter(h.kbc, msg.Sender.Username, msg.Channel); err != nil {
		return fmt.Errorf("error getting role status: %s", err)
	} else if !ok {
		h.ChatEcho(msg.ConvID, "you must be at least a writer to use this command")
		return nil
	}

	hostedURL, repo, err := parseRepoInput(args[0])
	if err != nil {
		h.ChatEcho(msg.ConvID, "Invalid repo: %q, expected `<owner/repo>` or `https://domain.com/owner/repo`", repo)
		return nil
	}

	subscriptionFound, reauthorizationNeeded, err := h.db.GetSubscriptionForRepoStatus(ctx, msg.ConvID, repo)
	if err != nil {
		return fmt.Errorf("error checking subscription: %s", err)
	}

	if create {
		if subscriptionFound && reauthorizationNeeded {
			_, err = h.kbc.SendMessageByTlfName(msg.Sender.Username, "%s", formatReauthorizationInstructions(repo, hostedURL, msg, h.httpPrefix, h.secret))
			if err != nil {
				return fmt.Errorf("error sending message: %s", err)
			}
			if !base.IsDirectPrivateMessage(h.kbc.GetUsername(), msg.Sender.Username, msg.Channel) {
				h.ChatEcho(msg.ConvID, "OK! I've sent instructions to @%s to reauthorize the webhook.", msg.Sender.Username)
			}
			return nil
		}
		if !subscriptionFound {
			err = h.db.CreateSubscription(ctx, msg.ConvID, repo, base.IdentifierFromMsg(msg))
			if err != nil {
				return fmt.Errorf("error creating subscription: %s", err)
			}
			_, err = h.kbc.SendMessageByTlfName(msg.Sender.Username, "%s", formatSetupInstructions(repo, hostedURL, msg, h.httpPrefix, h.secret))
			if err != nil {
				return fmt.Errorf("error sending message: %s", err)
			}
			if !base.IsDirectPrivateMessage(h.kbc.GetUsername(), msg.Sender.Username, msg.Channel) {
				h.ChatEcho(msg.ConvID, "OK! I've sent a message to @%s to authorize me.", msg.Sender.Username)
			}
			return nil
		}

		h.ChatEcho(msg.ConvID, "You're already receiving notifications for `%s` here!", repo)
		return nil
	}

	if subscriptionFound {
		err = h.db.DeleteSubscriptionsForRepo(ctx, msg.ConvID, repo)
		if err != nil {
			return fmt.Errorf("error deleting subscriptions: %s", err)
		}
		h.ChatEcho(msg.ConvID, "Okay, you won't receive updates for `%s` here.", repo)
		return nil
	}

	h.ChatEcho(msg.ConvID, "You aren't subscribed to updates for `%s`!", repo)
	return nil
}

func (h *Handler) handleListSubscriptions(ctx context.Context, msg chat1.MsgSummary) (err error) {
	subscriptions, err := h.db.GetAllSubscriptionsForConvID(ctx, msg.ConvID)
	if err != nil {
		return fmt.Errorf("error getting current repos: %s", err)
	}

	if len(subscriptions) == 0 {
		h.ChatEcho(msg.ConvID, "Not subscribed to any projects yet.")
		return nil
	}

	var res strings.Builder
	for _, repo := range subscriptions {
		res.WriteString(fmt.Sprintf("- *%s*\n", repo))
	}
	h.ChatEcho(msg.ConvID, "%s", res.String())
	return nil
}
