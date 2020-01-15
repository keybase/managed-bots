package gitlabbot

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/xanzy/go-gitlab"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/managed-bots/base"

	"github.com/google/go-github/v28/github"
)

func makeSecret(repo string, shortConvID base.ShortID, secret string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(repo+string(shortConvID)+secret)))
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

func formatPushMsg(evt *github.PushEvent, username string) (res string) {
	branch := refToName(evt.GetRef())

	res = fmt.Sprintf("%s pushed %d commit", username, len(evt.Commits))
	if len(evt.Commits) != 1 {
		res += "s"
	}
	res += fmt.Sprintf(" to %s/%s:\n", evt.GetRepo().GetName(), branch)
	for _, commit := range evt.Commits {
		res += fmt.Sprintf("- `%s`\n", formatCommitString(commit.GetMessage(), 50))
	}

	urlSplit := strings.Split(evt.GetCompare(), "://")
	if len(urlSplit) != 2 {
		// if the compare URL isn't formatted as expected, just skip it
		return res
	}
	res += fmt.Sprintf("\n%s", urlSplit[1])
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

func formatCheckSuiteMsg(evt *github.CheckSuiteEvent, username string) (res string) {
	action := evt.Action
	if *action == "completed" {
		suite := evt.GetCheckSuite()
		repo := evt.GetRepo().GetName()
		isPullRequest := len(suite.PullRequests) > 0
		switch suite.GetConclusion() {
		case "success":
			if !isPullRequest {
				res = fmt.Sprintf(":white_check_mark: Tests passed for %s/%s.", repo, suite.GetHeadBranch())
			} else {
				pr := suite.PullRequests[0]
				res = fmt.Sprintf(":white_check_mark: All tests passed for pull request #%d on %s.\n%s/pull/%d", pr.GetNumber(), repo, evt.GetRepo().GetHTMLURL(), pr.GetNumber())
			}
		case "failure", "timed_out", "action_required":
			if !isPullRequest {
				res = fmt.Sprintf(":x: Tests failed for %s/%s.", repo, suite.GetHeadBranch())
			} else {
				pr := suite.PullRequests[0]
				res = fmt.Sprintf(":x: Tests failed for pull request #%d on %s.\n%s/pull/%d", pr.GetNumber(), repo, evt.GetRepo().GetHTMLURL(), pr.GetNumber())
			}
		case "cancelled":
			if !isPullRequest {
				res = fmt.Sprintf(":warning: Tests cancelled for %s/%s.", repo, suite.GetHeadBranch())
			} else {
				pr := suite.PullRequests[0]
				res = fmt.Sprintf(":warning: Tests cancelled for pull request #%d on %s.\n%s/pull/%d", pr.GetNumber(), repo, evt.GetRepo().GetHTMLURL(), pr.GetNumber())
			}
		}
		if strings.HasPrefix(username, "@") && isPullRequest && res != "" {
			res = res + "\n" + username
		}
	}
	return res
}

func getDefaultBranch(repo string, client *gitlab.Client) (branch string, err error) {
	//args := strings.Split(repo, "/")
	//if len(args) != 2 {
	//	return "", fmt.Errorf("getDefaultBranch: invalid repo %s", repo)
	//}
	//
	//if client == nil {
	//	return "", fmt.Errorf("getDefaultBranch: client is nil")
	//}
	//
	//repoObject, res, err := client.Repositories.Get(context.TODO(), args[0], args[1])
	//if res.StatusCode == http.StatusNotFound {
	//	return "master", nil
	//}
	//if err != nil {
	//	return "", err
	//}
	//
	//return repoObject.GetDefaultBranch(), nil
	return "master", nil
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
