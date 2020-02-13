package gitlabbot

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/xanzy/go-gitlab"

	"github.com/keybase/managed-bots/base"
)

func getCommitMessages(event *gitlab.PushEvent) []string {
	var commitMsgs = make([]string, 0)
	for _, commit := range event.Commits {
		commitMsgs = append(commitMsgs, commit.Message)
	}
	return commitMsgs
}

// formatters

func formatPipelineMsg(evt *gitlab.PipelineEvent, username string) (res string) {
	action := evt.ObjectAttributes.Status
	suite := evt.ObjectAttributes
	repo := evt.Project.PathWithNamespace
	isMergeRequest := evt.MergeRequest.IID != 0
	pipelineURL := fmt.Sprintf("%s/pipelines/%d", evt.Project.WebURL, evt.ObjectAttributes.ID)

	defer func() {
		if strings.HasPrefix(username, "@") && isMergeRequest && res != "" {
			res = res + "\n" + username
		}
	}()
	switch action {
	case "success":
		if !isMergeRequest {
			return fmt.Sprintf(":white_check_mark: Tests passed for %s/%s.\n%s", repo, suite.Ref, pipelineURL)
		}
		mr := evt.MergeRequest
		return fmt.Sprintf(":white_check_mark: All tests passed for merge request #%d on %s.\n%s", mr.IID, repo, mr.URL)
	case "failed":
		if !isMergeRequest {
			return fmt.Sprintf(":x: Tests failed for %s/%s.\n%s", repo, suite.Ref, pipelineURL)
		}

		mr := evt.MergeRequest
		return fmt.Sprintf(":x: Tests failed for merge request #%d on %s.\n%s", mr.IID, repo, mr.URL)
	case "canceled":
		if !isMergeRequest {
			return fmt.Sprintf(":warning: Tests cancelled for %s/%s.\n%s", repo, suite.Ref, pipelineURL)
		}
		mr := evt.MergeRequest
		return fmt.Sprintf(":warning: Tests cancelled for pull request #%d on %s.\n%s", mr.IID, repo, mr.URL)
	default:
		return ""
	}
}

func formatSetupInstructions(repo string, hostedURL string, msg chat1.MsgSummary, httpAddress string, secret string) (res string) {
	back := "`"
	message := fmt.Sprintf(`
To configure your project to send notifications, go to %s/%s/-/settings/integrations and add a new webhook.
For “URL”, enter %s%s/gitlabbot/webhook%s.
For “Secret Token”, enter %s%s%s.
Remember to check all the triggers you would like me to update you on.
Note that I currently support the following Webhook Events: Push, Issues, Merge Request, Pipeline

Happy coding!`,
		hostedURL, repo, back, httpAddress, back, back, base.MakeSecret(repo, msg.ConvID, secret), back)
	return message
}

// parseRepoInput checks if url or <owner/repo> form
func parseRepoInput(urlOrRepoPath string) (hostedURL string, repo string) {
	parsedURL, err := url.ParseRequestURI(urlOrRepoPath)
	if err != nil {
		repo = urlOrRepoPath
		hostedURL = "https://gitlab.com"
	} else {
		hostedURL = parsedURL.Scheme + "://" + parsedURL.Host
		repo = parsedURL.Path[1:] // Remove the preceding slash '/owner/repo'
	}

	return hostedURL, repo
}
