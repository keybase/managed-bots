package githubbot

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/bradleyfalzon/ghinstallation"

	"github.com/google/go-github/v28/github"
	"github.com/kballard/go-shellquote"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"golang.org/x/oauth2"
)

type Handler struct {
	*base.DebugOutput

	kbc        *kbchat.API
	db         *DB
	requests   *base.OAuthRequests
	config     *oauth2.Config
	atr        *ghinstallation.AppsTransport
	httpPrefix string
	appName    string
	secret     string
}

var _ base.Handler = (*Handler)(nil)

func NewHandler(kbc *kbchat.API, db *DB, requests *base.OAuthRequests, config *oauth2.Config, atr *ghinstallation.AppsTransport, httpPrefix string, appName string, secret string) *Handler {
	return &Handler{
		DebugOutput: base.NewDebugOutput("Handler", kbc),
		kbc:         kbc,
		db:          db,
		requests:    requests,
		config:      config,
		atr:         atr,
		httpPrefix:  httpPrefix,
		appName:     appName,
		secret:      secret,
	}
}

func (h *Handler) HandleNewConv(conv chat1.ConvSummary) error {
	welcomeMsg := fmt.Sprintf(
		"Hi! I can notify you whenever something happens on a GitHub repository. To get started, install the Keybase integration on your repository, then send `!github subscribe <username/repo>`\n\ngithub.com/apps/%s/installations/new",
		h.appName,
	)
	return base.HandleNewTeam(h.DebugOutput, h.kbc, conv, welcomeMsg)
}

func (h *Handler) HandleAuth(msg chat1.MsgSummary, _ string) error {
	return h.HandleCommand(msg)
}

func (h *Handler) HandleCommand(msg chat1.MsgSummary) error {
	if msg.Content.Text == nil {
		h.Debug("skipping non-text message")
		return nil
	}

	cmd := strings.ToLower(strings.TrimSpace(msg.Content.Text.Body))
	if !strings.HasPrefix(cmd, "!github") {
		// ignore non-command message
		return nil
	}

	if strings.HasPrefix(cmd, "!github mentions") {
		// handle user preferences without needing oauth
		return h.handleMentionPref(cmd, msg)
	}

	client := github.NewClient(&http.Client{Transport: h.atr})

	switch {
	case strings.HasPrefix(cmd, "!github subscribe"):
		return h.handleSubscribe(cmd, msg, true, client)
	case strings.HasPrefix(cmd, "!github unsubscribe"):
		return h.handleSubscribe(cmd, msg, false, client)
	default:
		h.Debug("ignoring unknown command")
	}
	return nil
}

func (h *Handler) handleSubscribe(cmd string, msg chat1.MsgSummary, create bool, client *github.Client) (err error) {
	isAdmin, err := base.IsAdmin(h.kbc, msg)
	if err != nil {
		return fmt.Errorf("Error getting admin status: %s", err)
	}
	if !isAdmin {
		_, err = h.kbc.SendMessageByConvID(msg.ConvID, "You must be an admin to configure me!")
		if err != nil {
			err = fmt.Errorf("error sending message: %s", err)
		}
		return err
	}

	toks, err := shellquote.Split(cmd)
	if err != nil {
		return fmt.Errorf("error splitting command: %s", err)
	}

	args := toks[2:]
	if len(args) < 1 {
		return fmt.Errorf("bad args for subscribe: %v", args)
	}

	// Check if command is subscribing to a branch
	if len(args) == 2 {
		switch args[1] {
		case "issues", "pulls", "statuses", "commits":
			return h.handleSubscribeToFeature(args[0], args[1], msg, create)
		default:
			return h.handleSubscribeToBranch(args[0], args[1], msg, create)
		}
	}

	var message string
	defer func() {
		if message != "" {
			_, err = h.kbc.SendMessageByConvID(msg.ConvID, fmt.Sprintf(message, args[0]))
			if err != nil {
				err = fmt.Errorf("error sending message: %s", err)
			}
		}
	}()

	if err != nil {
		return fmt.Errorf("error getting default branch: %s", err)
	}
	alreadyExists, err := h.db.GetSubscriptionForRepoExists(msg.ConvID, args[0])
	if err != nil {
		return fmt.Errorf("error checking subscription: %s", err)
	}

	parsedRepo := strings.Split(args[0], "/")
	if len(parsedRepo) != 2 {
		if create {
			message = "`%s` doesn't look like a repository to me! Try sending `!github subscribe <owner/repository>`"
		} else {
			message = "`%s` doesn't look like a repository to me! Try sending `!github unsubscribe <owner/repository>`"
		}
		return nil
	}
	if create {
		if !alreadyExists {
			repoInstallation, res, err := client.Apps.FindRepositoryInstallation(context.TODO(), parsedRepo[0], parsedRepo[1])

			if err != nil {
				switch res.StatusCode {
				case http.StatusNotFound:
					message = "I couldn't subscribe to updates on %s! Make sure the app is installed on your repository, and that the repository exists."
					return nil
				default:
					return fmt.Errorf("error getting installation: %s", err)
				}
			}

			// check that user has authorization
			tc, err := base.GetOAuthClient(msg.Sender.Username, msg, h.kbc, h.requests, h.config, h.db,
				base.GetOAuthOpts{
					AuthMessageTemplate: "Visit %s\n to authorize me to set up GitHub updates.",
				})
			if err != nil || tc == nil {
				return err
			}
			userClient := github.NewClient(tc)
			installations, _, err := userClient.Apps.ListUserInstallations(context.TODO(), nil)
			if err != nil {
				return fmt.Errorf("Error getting installations for current user: %s", err)
			}

			// search through all user installations to see if they have permission to access the repo's installation
			hasPermission := false
			for _, i := range installations {
				if i.GetID() == repoInstallation.GetID() {
					hasPermission = true
				}
			}

			if !hasPermission {
				message = "You don't have permission to subscribe to %s."
				return nil
			}

			// auth checked, now we create the subscription
			err = h.db.CreateSubscription(msg.ConvID, args[0], repoInstallation.GetID())
			if err != nil {
				return fmt.Errorf("error creating subscription: %s", err)
			}
			message = "Okay, you'll receive updates for %s here."
			return nil
		}

		message = "You're already receiving notifications for %s here!"
		return nil
	}

	// unsubscribing
	if alreadyExists {

		err = h.db.DeleteSubscriptionsForRepo(msg.ConvID, args[0])
		if err != nil {
			return fmt.Errorf("error deleting subscriptions: %s", err)
		}

		err = h.db.DeleteBranchesForRepo(msg.ConvID, args[0])
		if err != nil {
			return fmt.Errorf("error deleting branches: %s", err)
		}

		err = h.db.DeleteFeaturesForRepo(msg.ConvID, args[0])
		if err != nil {
			return fmt.Errorf("error deleting features: %s", err)
		}
		message = "Okay, you won't receive updates for %s here."
		return nil
	}

	message = "You aren't subscribed to updates for %s!"
	return nil
}

func (h *Handler) handleSubscribeToFeature(repo string, feature string, msg chat1.MsgSummary, enable bool) (err error) {
	// isAdmin is checked in handleSubscribe
	var message string
	defer func() {
		if message != "" {
			_, err = h.kbc.SendMessageByConvID(msg.ConvID, fmt.Sprintf(message, feature, repo))
			if err != nil {
				err = fmt.Errorf("error sending message: %s", err)
			}
		}
	}()

	if exists, err := h.db.GetSubscriptionForRepoExists(msg.ConvID, repo); !exists {
		if err != nil {
			return fmt.Errorf("error getting subscription: %s", err)
		}
		var message string
		if enable {
			message = fmt.Sprintf("You aren't subscribed to updates yet!\nSend this first: `!github subscribe %s`", repo)
		} else {
			message = fmt.Sprintf("You aren't subscribed to notifications for %s!", repo)
		}
		_, err := h.kbc.SendMessageByConvID(msg.ConvID, message)
		if err != nil {
			return fmt.Errorf("Error sending message: %s", err)
		}
		return nil
	}

	currentFeatures, err := h.db.GetFeatures(msg.ConvID, repo)
	if err != nil {
		return fmt.Errorf("Error getting current features: %s", err)
	}
	// "issues", "pulls", "statuses", "commits"
	switch feature {
	case "issues":
		currentFeatures.Issues = enable
	case "pulls":
		currentFeatures.PullRequests = enable
	case "statuses":
		currentFeatures.Statuses = enable
	case "commits":
		currentFeatures.Commits = enable
	default:
		// Should never get here if check in handleSubscribe is correct
		return fmt.Errorf("Error subscribing to feature: %s is not a valid feature", feature)
	}

	if enable {
		message = "Okay, you'll receive notifications for %s on %s!"
	} else {
		message = "Okay, you won't receive notifications for %s for %s."
	}
	err = h.db.SetFeatures(msg.ConvID, repo, currentFeatures)
	if err != nil {
		return fmt.Errorf("Error setting features: %s", err)
	}
	return nil
}

func (h *Handler) handleSubscribeToBranch(repo string, branch string, msg chat1.MsgSummary, create bool) (err error) {
	// isAdmin is checked in handleSubscribe
	var message string
	defer func() {
		if message != "" {
			_, err = h.kbc.SendMessageByConvID(msg.ConvID, fmt.Sprintf(message, repo, branch))
			if err != nil {
				err = fmt.Errorf("error sending message: %s", err)
			}
		}
	}()

	if exists, err := h.db.GetSubscriptionForRepoExists(msg.ConvID, repo); !exists {
		if err != nil {
			return fmt.Errorf("error getting subscription: %s", err)
		}
		var message string
		if create {
			message = fmt.Sprintf("You aren't subscribed to updates yet!\nSend this first: `!github subscribe %s`", repo)
		} else {
			message = fmt.Sprintf("You aren't subscribed to notifications for %s!", repo)
		}
		_, err := h.kbc.SendMessageByConvID(msg.ConvID, message)
		if err != nil {
			return fmt.Errorf("Error sending message: %s", err)
		}
		return nil
	}

	if create {
		err = h.db.WatchBranch(msg.ConvID, repo, branch)
		if err != nil {
			return fmt.Errorf("error creating branch subscription: %s", err)
		}

		message = "Now subscribed to commits on %s/%s."
		return nil
	}
	err = h.db.UnwatchBranch(msg.ConvID, repo, branch)
	if err != nil {
		return fmt.Errorf("error deleting branch subscription: %s", err)
	}

	message = "Okay, you won't receive notifications for commits in %s/%s."
	return nil
}

// user preferences
func (h *Handler) handleMentionPref(cmd string, msg chat1.MsgSummary) (err error) {
	var message string
	defer func() {
		if message != "" {
			_, err = h.kbc.SendMessageByConvID(msg.ConvID, message)
			if err != nil {
				err = fmt.Errorf("error sending message: %s", err)
			}
		}
	}()

	toks, err := shellquote.Split(cmd)
	if err != nil {
		return fmt.Errorf("error splitting command: %s", err)
	}
	args := toks[2:]
	if len(args) != 1 || (args[0] != "disable" && args[0] != "enable") {
		message = "I don't understand! Try `!github mentions disable` or `!github mentions enable`."
		return nil
	}

	allowMentions := args[0] == "enable"
	err = h.db.SetUserPreferences(msg.Sender.Username, &UserPreferences{Mention: allowMentions})
	if err != nil {
		return fmt.Errorf("error setting user preference: %s", err)
	}

	if allowMentions {
		message = "Okay, you'll be mentioned in GitHub events involving your linked GitHub account."
	} else {
		message = "Okay, you won't be mentioned in future GitHub events."
	}

	return
}
