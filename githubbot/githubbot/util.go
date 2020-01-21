package githubbot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/managed-bots/base"

	"github.com/google/go-github/v28/github"
)

func refToName(ref string) (branch string) {
	// refs are always given in the form "refs/heads/{branch name}" or "refs/tags/{tag name}"
	branch = strings.Split(ref, "refs/")[1]
	if strings.HasPrefix(branch, "heads/") {
		branch = strings.Split(branch, "heads/")[1]
	}
	// if we got a tag ref, just leave it as "tags/{tag name}"
	return branch
}

func getCommitMessages(event *github.PushEvent) []string {
	var commitMsgs = make([]string, 0)
	for _, commit := range event.Commits {
		commitMsgs = append(commitMsgs, commit.GetMessage())
	}
	return commitMsgs
}

// formatters

func formatIssueMsg(evt *github.IssuesEvent, username string) (res string) {
	action := evt.Action
	if action != nil {
		switch *action {
		case "opened":
			res = fmt.Sprintf("%s opened issue #%d on %s: “%s”\n", username, evt.GetIssue().GetNumber(), evt.GetRepo().GetName(), evt.GetIssue().GetTitle())
			res += evt.GetIssue().GetHTMLURL()
		case "closed":
			res = fmt.Sprintf("%s closed issue #%d on %s.\n", username, evt.GetIssue().GetNumber(), evt.GetRepo().GetName())
			res += evt.GetIssue().GetHTMLURL()
		}
	}
	return res
}

func formatPRMsg(evt *github.PullRequestEvent, username string) (res string) {
	action := evt.Action
	if action != nil {
		switch *action {
		case "opened":
			res = fmt.Sprintf("%s opened pull request #%d on %s: “%s”\n", username, evt.GetNumber(), evt.GetRepo().GetName(), evt.GetPullRequest().GetTitle())
			res += evt.GetPullRequest().GetHTMLURL()
		case "closed":
			if evt.GetPullRequest().GetMerged() {
				// PR was merged
				res = fmt.Sprintf("%s merged pull request #%d into %s/%s.\n", username, evt.GetNumber(), evt.GetRepo().GetName(), evt.GetPullRequest().GetBase().GetRef())
				res += evt.GetPullRequest().GetHTMLURL()
			} else {
				// PR was closed without merging
				res = fmt.Sprintf("%s closed pull request #%d on %s.\n", username, evt.GetNumber(), evt.GetRepo().GetName())
				res += evt.GetPullRequest().GetHTMLURL()
			}
		}
	}
	return res
}

func formatCheckRunMessage(evt *github.CheckRunEvent, username string) (res string) {
	action := evt.Action
	if *action == "completed" {
		run := evt.GetCheckRun()
		repo := evt.GetRepo().GetName()
		isPullRequest := len(run.PullRequests) > 0
		var testName string
		if run.GetName() != "" {
			testName = fmt.Sprintf("*%s*", run.GetName())
		} else {
			testName = "Test"
		}
		switch run.GetConclusion() {
		case "success":
			if !isPullRequest {
				res = fmt.Sprintf(":white_check_mark: %s passed for %s/%s.", testName, repo, run.GetCheckSuite().GetHeadBranch())
			} else {
				pr := run.PullRequests[0]
				res = fmt.Sprintf(":white_check_mark: %s passed for pull request #%d on %s.\n%s/pull/%d", testName, pr.GetNumber(), repo, evt.GetRepo().GetHTMLURL(), pr.GetNumber())
			}
		case "failure", "timed_out", "action_required":
			urlSplit := strings.Split(run.GetHTMLURL(), "://")
			var targetURL string
			if len(urlSplit) != 2 {
				// if the CI target URL isn't formatted as expected, just skip it
				targetURL = ""
			} else {
				targetURL = urlSplit[1]
			}

			if !isPullRequest {
				res = fmt.Sprintf(":x: %s failed for %s/%s.\n%s", testName, repo, run.GetCheckSuite().GetHeadBranch(), targetURL)
			} else {
				pr := run.PullRequests[0]
				res = fmt.Sprintf(":x: %s failed for pull request #%d on %s.\n%s", testName, pr.GetNumber(), repo, targetURL)
			}
		case "cancelled":
			if !isPullRequest {
				res = fmt.Sprintf(":warning: %s cancelled for %s/%s.", testName, repo, run.GetCheckSuite().GetHeadBranch())
			} else {
				pr := run.PullRequests[0]
				res = fmt.Sprintf(":warning: %s cancelled for pull request #%d on %s.\n%s/pull/%d", testName, pr.GetNumber(), repo, evt.GetRepo().GetHTMLURL(), pr.GetNumber())
			}
		}
		if strings.HasPrefix(username, "@") && isPullRequest && res != "" {
			res = fmt.Sprintf("%s\n%s", res, username)
		}
	}
	return res
}

func formatStatusMessage(evt *github.StatusEvent, pullRequests []*github.PullRequest, username string) (res string) {
	state := evt.GetState()
	repo := evt.GetRepo().GetName()
	isPullRequest := len(pullRequests) > 0
	var branch string
	if len(evt.Branches) < 1 {
		return ""
	}
	branch = evt.Branches[0].GetName()
	var testName string
	if evt.GetContext() != "" {
		testName = fmt.Sprintf("*%s*", evt.GetContext())
	} else {
		testName = "Test"
	}
	switch state {
	case "success":
		if !isPullRequest {
			res = fmt.Sprintf(":white_check_mark: %s passed for %s/%s.", testName, repo, branch)
		} else {
			pr := pullRequests[0]
			res = fmt.Sprintf(":white_check_mark: %s passed for pull request #%d on %s.\n%s/pull/%d", testName, pr.GetNumber(), repo, evt.GetRepo().GetHTMLURL(), pr.GetNumber())
		}
	case "failure", "error":
		var targetURL string
		urlSplit := strings.Split(evt.GetTargetURL(), "://")
		if len(urlSplit) != 2 {
			// if the CI target URL isn't formatted as expected, just skip it
			targetURL = ""
		} else {
			targetURL = urlSplit[1]
		}

		if !isPullRequest {
			res = fmt.Sprintf(":x: %s failed for %s/%s.\n%s", testName, repo, branch, targetURL)
		} else {
			pr := pullRequests[0]

			res = fmt.Sprintf(":x: %s failed for pull request #%d on %s.\n%s", testName, pr.GetNumber(), repo, targetURL)
		}
	}
	if strings.HasPrefix(username, "@") && isPullRequest && res != "" {
		res = fmt.Sprintf("%s\n%s", res, username)
	}
	return res
}

func getDefaultBranch(repo string, client *github.Client) (branch string, err error) {
	args := strings.Split(repo, "/")
	if len(args) != 2 {
		return "", fmt.Errorf("getDefaultBranch: invalid repo %s", repo)
	}

	if client == nil {
		return "", fmt.Errorf("getDefaultBranch: client is nil")
	}

	repoObject, res, err := client.Repositories.Get(context.TODO(), args[0], args[1])
	if err != nil {
		return "", err
	}
	if res.StatusCode == http.StatusNotFound {
		return "master", nil
	}

	return repoObject.GetDefaultBranch(), nil
}

// keybase IDing

type username struct {
	githubUsername  string
	keybaseUsername *string
}

func (u username) String() string {
	if u.keybaseUsername != nil {
		return "@" + *u.keybaseUsername
	}

	return fmt.Sprintf("*%s*", u.githubUsername)
}

type keybaseID struct {
	Username string `json:"username"`
}

func getPossibleKBUser(kbc *kbchat.API, d *DB, debug *base.DebugOutput, githubUsername string) (u username) {
	u = username{githubUsername: githubUsername}
	id := kbc.Command("id", "-j", fmt.Sprintf("%s@github", githubUsername))
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
