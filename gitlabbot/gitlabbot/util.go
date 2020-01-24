package gitlabbot

import (
	"encoding/json"
	"fmt"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/xanzy/go-gitlab"
	"net/http"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
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
	pipelineURL := fmt.Sprintf("https://gitlab.com/%s/pipelines/%d", repo, evt.ObjectAttributes.ID)
	switch action {
	case "success":
		if !isMergeRequest {
			res = fmt.Sprintf(":white_check_mark: Tests passed for %s/%s.\n%s", repo, suite.Ref, pipelineURL)
		} else {
			mr := evt.MergeRequest
			res = fmt.Sprintf(":white_check_mark: All tests passed for merge request #%d on %s.\n%s", mr.IID, repo, mr.URL)
		}
	case "failed":
		if !isMergeRequest {
			res = fmt.Sprintf(":x: Tests failed for %s/%s.\n%s", repo, suite.Ref, pipelineURL)
		} else {
			mr := evt.MergeRequest
			res = fmt.Sprintf(":x: Tests failed for merge request #%d on %s.\n%s", mr.IID, repo, mr.URL)
		}
	case "canceled":
		if !isMergeRequest {
			res = fmt.Sprintf(":warning: Tests cancelled for %s/%s.\n%s", repo, suite.Ref, pipelineURL)
		} else {
			mr := evt.MergeRequest
			res = fmt.Sprintf(":warning: Tests cancelled for pull request #%d on %s.\n%s", mr.IID, repo, mr.URL)
		}
	}
	if strings.HasPrefix(username, "@") && isMergeRequest && res != "" {
		res = res + "\n" + username
	}
	return res
}

func formatSetupInstructions(repo string, msg chat1.MsgSummary, httpAddress string, secret string) (res string) {
	back := "`"
	message := fmt.Sprintf(`
To configure your project to send notifications, go to https://gitlab.com/%s/-/settings/integrations and add a new webhook.
For “URL”, enter %s%s/gitlabbot/webhook%s.
For “Secret Token”, enter %s%s%s.
Remember to check all the triggers you would like me to update you on.
Note that I currently support the following Webhook Events: Push, Issues, Merge Request, Pipeline

Happy coding!`,
		repo, back, httpAddress, back, back, base.MakeSecret(repo, msg.ConvID, secret), back)
	return message
}

func getProject(repo string, client *gitlab.Client) (*gitlab.Project, error) {
	if client == nil {
		return nil, fmt.Errorf("getDefaultBranch: client is nil")
	}

	project, res, err := client.Projects.GetProject(repo, &gitlab.GetProjectOptions{})
	if err != nil || res.StatusCode != http.StatusOK {
		return nil, err
	}

	return project, nil
}

// keybase IDing

type username struct {
	gitlabUsername  string
	keybaseUsername *string
}

func (u username) String() string {
	if u.keybaseUsername != nil {
		return "@" + *u.keybaseUsername
	}

	return fmt.Sprintf("*%s*", u.gitlabUsername)
}

type keybaseID struct {
	Username string `json:"username"`
}

// Most of the logic here is useless until there is a Keybase GitLab proof
// but keeping it here for abstraction reasons still
func getPossibleKBUser(kbc *kbchat.API, debug *base.DebugOutput, gitlabUsername string) (u username) {
	u = username{gitlabUsername: gitlabUsername}
	id := kbc.Command("id", "-j", fmt.Sprintf("%s@gitlab", gitlabUsername))
	output, err := id.Output()
	if err != nil {
		// fall back to github username if `keybase id` errors
		return u
	}

	var i keybaseID
	err = json.Unmarshal(output, &i)
	if err != nil {
		debug.Debug("getPossibleKBUser: couldn't parse keybase id: %s", err)
		return u
	}

	return u
}
