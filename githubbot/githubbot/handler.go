package githubbot

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/bradleyfalzon/ghinstallation"

	"github.com/google/go-github/v31/github"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"golang.org/x/oauth2"
)

type Handler struct {
	*base.DebugOutput

	stats       *base.StatsRegistry
	kbc         *kbchat.API
	db          *DB
	oauthConfig *oauth2.Config
	atr         *ghinstallation.AppsTransport
	httpPrefix  string
	appName     string
}

var _ base.Handler = (*Handler)(nil)

func NewHandler(stats *base.StatsRegistry, kbc *kbchat.API, debugConfig *base.ChatDebugOutputConfig, db *DB,
	oauthConfig *oauth2.Config, atr *ghinstallation.AppsTransport,
	httpPrefix, appName string) *Handler {
	return &Handler{
		DebugOutput: base.NewDebugOutput("Handler", debugConfig),
		stats:       stats.SetPrefix("Handler"),
		kbc:         kbc,
		db:          db,
		oauthConfig: oauthConfig,
		atr:         atr,
		httpPrefix:  httpPrefix,
		appName:     appName,
	}
}

func (h *Handler) HandleNewConv(conv chat1.ConvSummary) error {
	welcomeMsg := fmt.Sprintf(
		"Hi! I can notify you whenever something happens on a GitHub repository. To get started, install the Keybase integration on your repository, then send `!github subscribe <owner/repo>`\n\ngithub.com/apps/%s/installations/new",
		h.appName,
	)
	return base.HandleNewTeam(h.stats, h.DebugOutput, h.kbc, conv, welcomeMsg)
}

func (h *Handler) HandleAuth(msg chat1.MsgSummary, _ string) error {
	return h.HandleCommand(msg)
}

func (h *Handler) HandleCommand(msg chat1.MsgSummary) error {
	if msg.Content.Text == nil {
		return nil
	}

	cmd := strings.ToLower(strings.TrimSpace(msg.Content.Text.Body))
	if !strings.HasPrefix(cmd, "!github") {
		// ignore non-command message
		return nil
	}

	if strings.HasPrefix(cmd, "!github mentions") {
		// handle user preferences without needing oauth
		h.stats.Count("mentions")
		return h.handleMentionPref(cmd, msg)
	}

	client := github.NewClient(&http.Client{Transport: h.atr})
	switch {
	case strings.HasPrefix(cmd, "!github subscribe"):
		h.stats.Count("subscribe")
		return h.handleSubscribe(cmd, msg, true, client)
	case strings.HasPrefix(cmd, "!github unsubscribe"):
		h.stats.Count("unsubscribe")
		return h.handleSubscribe(cmd, msg, false, client)
	case strings.HasPrefix(cmd, "!github list"):
		h.stats.Count("list")
		return h.handleListSubscriptions(msg)
	default:
		h.Debug("ignoring unknown command %q", cmd)
	}
	return nil
}

func (h *Handler) handleSubscribe(cmd string, msg chat1.MsgSummary, create bool, client *github.Client) (err error) {
	toks, userErr, err := base.SplitTokens(cmd)
	if err != nil {
		return err
	} else if userErr != "" {
		h.ChatEcho(msg.ConvID, "%s", userErr)
		return nil
	}

	args := toks[2:]
	if len(args) < 1 {
		if create {
			h.ChatEcho(msg.ConvID, "I don't understand! Try `!github subscribe <owner/repo>`")
		} else {
			h.ChatEcho(msg.ConvID, "I don't understand! Try `!github unsubscribe <owner/repo>`")
		}
		return nil
	}

	isAllowed, err := base.IsAtLeastWriter(h.kbc, msg.Sender.Username, msg.Channel)
	if err != nil {
		return fmt.Errorf("Error getting role status: %s", err)
	}
	if !isAllowed {
		h.ChatEcho(msg.ConvID, "You must be at least a writer to configure me!")
		return nil
	}

	repo := args[0]
	// Check if command is subscribing to a branch
	alreadyExists, err := h.db.GetSubscriptionForRepoExists(msg.ConvID, repo)
	if err != nil {
		return fmt.Errorf("error checking subscription: %s", err)
	}
	if len(args) == 2 {
		if !alreadyExists {
			if create {
				if created, err := h.handleNewSubscription(repo, msg, client); err != nil {
					if _, ok := err.(base.OAuthRequiredError); ok {
						return nil
					}
					return err
				} else if !created {
					return nil
				}
			} else {
				h.ChatEcho(msg.ConvID, "You aren't subscribed to notifications for `%s`!", repo)
				return nil
			}
		}
		switch args[1] {
		case "issues", "pulls", "statuses", "commits", "releases":
			return h.handleSubscribeToFeature(repo, args[1], msg, create)
		default:
			return h.handleSubscribeToBranch(repo, args[1], msg, create)
		}
	}

	if create {
		if alreadyExists {
			h.ChatEcho(msg.ConvID, "You're already receiving notifications for `%s` here!", repo)
			return nil
		}
		created, err := h.handleNewSubscription(repo, msg, client)
		if err != nil {
			if _, ok := err.(base.OAuthRequiredError); ok {
				return nil
			}
			return err
		} else if !created {
			return nil
		}
		h.ChatEcho(msg.ConvID, "Okay, you'll receive updates for `%s` here.", repo)
		return nil
	}

	// unsubscribing
	if !alreadyExists {
		h.ChatEcho(msg.ConvID, "You aren't subscribed to updates for `%s`!", repo)
		return nil
	}

	err = h.db.DeleteSubscriptionsForRepo(msg.ConvID, repo)
	if err != nil {
		return fmt.Errorf("error deleting subscriptions: %s", err)
	}

	err = h.db.DeleteBranchesForRepo(msg.ConvID, repo)
	if err != nil {
		return fmt.Errorf("error deleting branches: %s", err)
	}

	err = h.db.DeleteFeaturesForRepo(msg.ConvID, repo)
	if err != nil {
		return fmt.Errorf("error deleting features: %s", err)
	}
	h.ChatEcho(msg.ConvID, "Okay, you won't receive updates for `%s` here.", repo)
	return nil
}

func (h *Handler) handleListSubscriptions(msg chat1.MsgSummary) (err error) {
	features, err := h.db.GetFeaturesForAllRepos(msg.ConvID)
	if err != nil {
		return fmt.Errorf("Error getting current features: %s", err)
	}

	if len(features) == 0 {
		h.ChatEcho(msg.ConvID, "Not subscribed to any repos yet.")
		return nil
	}

	var res string
	for repo, f := range features {
		res += fmt.Sprintf("- *%s* (%s)\n", repo, &f)
		if f.Commits {
			branches, err := h.db.GetAllBranchesForRepo(msg.ConvID, repo)
			if err != nil {
				return fmt.Errorf("error getting branches for repo: %s", err)
			}

			for _, branch := range branches {
				res += fmt.Sprintf("   - %s\n", branch)
			}
		}
	}
	h.ChatEcho(msg.ConvID, "%s", res)
	return nil
}

func (h *Handler) handleNewSubscription(repo string, msg chat1.MsgSummary, client *github.Client) (created bool, err error) {
	parsedRepo := strings.Split(repo, "/")
	if len(parsedRepo) != 2 {
		h.ChatEcho(msg.ConvID, "`%s` doesn't look like a repository to me! Try sending `!github subscribe <owner/repo>`", repo)
		return false, nil
	}
	repoInstallation, res, err := client.Apps.FindRepositoryInstallation(context.TODO(), parsedRepo[0], parsedRepo[1])
	if err != nil {
		switch res.StatusCode {
		case http.StatusNotFound:
			h.ChatEcho(msg.ConvID, "I couldn't subscribe to updates on `%s`! Make sure the Keybase integration is installed on your repository, and that the repository exists.\n\ngithub.com/apps/%s/installations/new", repo, h.appName)
			return false, nil
		default:
			return false, fmt.Errorf("error getting installation: %s", err)
		}
	}

	// check that user has authorization
	tc, err := base.GetOAuthClient(msg.Sender.Username, msg, h.kbc, h.oauthConfig, h.db,
		base.GetOAuthOpts{
			AuthMessageTemplate: "Authorize me by clicking this link:\n%s",
		})
	if err != nil || tc == nil {
		return false, err
	}
	userClient := github.NewClient(tc)
	installations, _, err := userClient.Apps.ListUserInstallations(context.TODO(), nil)
	if err != nil {
		return false, fmt.Errorf("Error getting installations for current user: %s", err)
	}

	// search through all user installations to see if they have permission to access the repo's installation
	hasPermission := false
	for _, i := range installations {
		if i.GetID() == repoInstallation.GetID() {
			hasPermission = true
			break
		}
	}

	if !hasPermission {
		h.ChatEcho(msg.ConvID, "You don't have permission to subscribe to `%s`.", repo)
		return false, fmt.Errorf("unauthorized for subscription")
	}

	// auth checked, now we create the subscription
	defaultBranch, err := GetDefaultBranch(repo, userClient)
	if err != nil {
		return false, fmt.Errorf("error getting default branch: %s", err)
	}

	err = h.db.WatchBranch(msg.ConvID, repo, defaultBranch)
	if err != nil {
		return false, fmt.Errorf("error watching branch: %s", err)
	}

	err = h.db.CreateSubscription(msg.ConvID, repo, repoInstallation.GetID())
	if err != nil {
		return false, fmt.Errorf("error creating subscription: %s", err)
	}
	return true, nil
}

func (h *Handler) handleSubscribeToFeature(repo, feature string, msg chat1.MsgSummary, enable bool) (err error) {
	exists, err := h.db.GetSubscriptionForRepoExists(msg.ConvID, repo)
	if err != nil {
		return fmt.Errorf("error getting subscription: %s", err)
	} else if !exists {
		if enable {
			h.ChatEcho(msg.ConvID, "You aren't subscribed to updates yet!\nSend this first: `!github subscribe %s`", repo)
		} else {
			h.ChatEcho(msg.ConvID, "You aren't subscribed to notifications for `%s`!", repo)
		}
		return nil
	}

	currentFeatures, err := h.db.GetFeatures(msg.ConvID, repo)
	if err != nil {
		return fmt.Errorf("Error getting current features: %s", err)
	}
	if currentFeatures == nil {
		currentFeatures = &Features{}
	}
	// "issues", "pulls", "statuses", "commits", "releases"
	switch feature {
	case "issues":
		currentFeatures.Issues = enable
	case "pulls":
		currentFeatures.PullRequests = enable
	case "statuses":
		currentFeatures.Statuses = enable
	case "commits":
		currentFeatures.Commits = enable
	case "releases":
		currentFeatures.Releases = enable
	default:
		// Should never get here if check in handleSubscribe is correct
		return fmt.Errorf("Error subscribing to feature: %s is not a valid feature", feature)
	}

	err = h.db.SetFeatures(msg.ConvID, repo, currentFeatures)
	if err != nil {
		return fmt.Errorf("Error setting features: %s", err)
	}
	if enable {
		h.ChatEcho(msg.ConvID, "Okay, you'll receive notifications for `%s` on `%s`!", feature, repo)
	} else {
		h.ChatEcho(msg.ConvID, "Okay, you won't receive notifications for `%s` for `%s`.", repo, feature)
	}
	return nil
}

func (h *Handler) handleSubscribeToBranch(repo, branch string, msg chat1.MsgSummary, create bool) (err error) {
	exists, err := h.db.GetSubscriptionForRepoExists(msg.ConvID, repo)
	if err != nil {
		return fmt.Errorf("error getting subscription: %s", err)
	} else if !exists {
		if create {
			h.ChatEcho(msg.ConvID, "You aren't subscribed to updates yet!\nSend this first: `!github subscribe %s`", repo)
		} else {
			h.ChatEcho(msg.ConvID, "You aren't subscribed to notifications for `%s`!", repo)
		}
		return nil
	}

	if create {
		err = h.db.WatchBranch(msg.ConvID, repo, branch)
		if err != nil {
			return fmt.Errorf("error creating branch subscription: %s", err)
		}

		h.ChatEcho(msg.ConvID, "Now subscribed to notifications for `%s/%s`.", repo, branch)
		return nil
	}
	err = h.db.UnwatchBranch(msg.ConvID, repo, branch)
	if err != nil {
		return fmt.Errorf("error deleting branch subscription: %s", err)
	}

	h.ChatEcho(msg.ConvID, "Okay, you won't receive notifications for `%s/%s`.", repo, branch)
	return nil
}

// user preferences
func (h *Handler) handleMentionPref(cmd string, msg chat1.MsgSummary) (err error) {
	toks, userErr, err := base.SplitTokens(cmd)
	if err != nil {
		return err
	} else if userErr != "" {
		h.ChatEcho(msg.ConvID, "%s", userErr)
		return nil
	}
	args := toks[2:]
	if len(args) != 1 || (args[0] != "disable" && args[0] != "enable") {
		h.ChatEcho(msg.ConvID, "I don't understand! Try `!github mentions disable` or `!github mentions enable`.")
		return nil
	}

	allowMentions := args[0] == "enable"
	err = h.db.SetUserPreferences(msg.Sender.Username, msg.ConvID, &UserPreferences{Mention: allowMentions})
	if err != nil {
		return fmt.Errorf("error setting user preference: %s", err)
	}

	if allowMentions {
		h.ChatEcho(msg.ConvID, "Okay, you'll be mentioned in GitHub events involving your linked GitHub account in this conversation.")
	} else {
		h.ChatEcho(msg.ConvID, "Okay, you won't be mentioned in future GitHub events in this conversation.")
	}
	return nil
}
