package gitlabbot

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/xanzy/go-gitlab"
	"net/http"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/managed-bots/base"
)

func makeSecret(repo string, convID chat1.ConvIDStr, secret string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(repo+string(base.ShortConvID(convID))+secret)))
}

func refToName(ref string) (branch string) {
	// refs are always given in the form "refs/heads/{branch name}" or "refs/tags/{tag name}"
	branch = strings.Split(ref, "refs/")[1]
	if strings.HasPrefix(branch, "heads/") {
		branch = strings.Split(branch, "heads/")[1]
	}
	// if we got a tag ref, just leave it as "tags/{tag name}"
	return branch
}

// formatters

func formatPushMsg(evt *gitlab.PushEvent, username string) (res string) {
	branch := refToName(evt.Ref)

	res = fmt.Sprintf("%s pushed %d commit", username, len(evt.Commits))
	if len(evt.Commits) != 1 {
		res += "s"
	}
	res += fmt.Sprintf(" to %s/%s:\n", evt.Project.Name, branch)
	for _, commit := range evt.Commits {
		res += fmt.Sprintf("- `%s`\n", formatCommitString(commit.Message, 50))
	}

	var lastCommitDiffURL = evt.Commits[len(evt.Commits) - 1].URL
	res += fmt.Sprintf("\n%s", lastCommitDiffURL)
	return res
}

func formatCommitString(commit string, maxLen int) string {
	firstLine := strings.Split(commit, "\n")[0]
	if len(firstLine) > maxLen {
		firstLine = strings.TrimSpace(firstLine[:maxLen]) + "..."
	}
	return firstLine
}

func formatIssueMsg(evt *gitlab.IssueEvent, username string) (res string) {
	action := evt.ObjectAttributes.Action
	switch action {
	case "open":
		res = fmt.Sprintf("%s opened issue #%d on %s: “%s”\n", username, evt.ObjectAttributes.IID, evt.Project.Name, evt.ObjectAttributes.Title)
		res += evt.ObjectAttributes.URL
	case "reopen":
		res = fmt.Sprintf("%s reopened issue #%d on %s: “%s”\n", username, evt.ObjectAttributes.IID, evt.Project.Name, evt.ObjectAttributes.Title)
		res += evt.ObjectAttributes.URL
	case "close":
		res = fmt.Sprintf("%s closed issue #%d on %s.\n", username, evt.ObjectAttributes.IID, evt.Project.Name)
		res += evt.ObjectAttributes.URL
	}
	return res
}

func formatMRMsg(evt *gitlab.MergeEvent, username string) (res string) {
	action := evt.ObjectAttributes.Action
	switch action {
	case "open":
		res = fmt.Sprintf("%s opened merge request #%d on %s: “%s”\n", username, evt.ObjectAttributes.IID, evt.Project.PathWithNamespace, evt.ObjectAttributes.Title)
		res += evt.ObjectAttributes.URL
	case "reopen":
		res = fmt.Sprintf("%s reopened merge request #%d on %s: “%s”\n", username, evt.ObjectAttributes.IID, evt.Project.PathWithNamespace, evt.ObjectAttributes.Title)
		res += evt.ObjectAttributes.URL
	case "close":
		res = fmt.Sprintf("%s closed merge request #%d on %s.\n", username, evt.ObjectAttributes.IID, evt.Project.PathWithNamespace)
		res += evt.ObjectAttributes.URL
	case "merge":
		res = fmt.Sprintf("%s merged merge request #%d into %s/%s.\n", username, evt.ObjectAttributes.IID, evt.Project.Name, evt.ObjectAttributes.TargetBranch)
		res += evt.ObjectAttributes.URL
	}
	return res
}

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

	return u.gitlabUsername
}

type keybaseID struct {
	Username string `json:"username"`
}

func getPossibleKBUser(kbc *kbchat.API, d *DB, debug *base.DebugOutput, gitlabUsername string) (u username) {
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

	prefs, err := d.GetUserPreferences(i.Username)
	if err != nil {
		debug.Debug("getPossibleKBUser: couldn't get user preferences: %s", err)
		return u
	}

	if prefs.Mention {
		u.keybaseUsername = &i.Username
	}

	return u
}
