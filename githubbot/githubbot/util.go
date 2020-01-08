package githubbot

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
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
				res = fmt.Sprintf("%s merged pull request #%d into %s/%s.\n", evt.GetPullRequest().GetMergedBy().GetLogin(), evt.GetNumber(), evt.GetRepo().GetName(), evt.GetPullRequest().GetBase().GetRef())
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
				break
			}
			pr := suite.PullRequests[0]
			res = fmt.Sprintf(":white_check_mark: All tests passed for pull request #%d on %s.\n%s/pull/%d", pr.GetNumber(), repo, evt.GetRepo().GetHTMLURL(), pr.GetNumber())
		case "failure", "timed_out":
			if !isPullRequest {
				res = fmt.Sprintf(":x: Tests failed for %s/%s.", repo, suite.GetHeadBranch())
				break
			}
			pr := suite.PullRequests[0]
			res = fmt.Sprintf(":x: Tests failed for pull request #%d on %s.\n%s/pull/%d", pr.GetNumber(), repo, evt.GetRepo().GetHTMLURL(), pr.GetNumber())
		}
		if strings.HasPrefix(username, "@") && isPullRequest {
			res = res + "\n" + username
		}
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
	if res.StatusCode == http.StatusNotFound {
		return "master", nil
	}
	if err != nil {
		return "", err
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

	return u.githubUsername
}

type keybaseID struct {
	Username string `json:"username"`
}

func getPossibleKBUser(kbc *kbchat.API, debug *base.DebugOutput, githubUsername string) (u username) {
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
	u.keybaseUsername = &i.Username

	return u
}
