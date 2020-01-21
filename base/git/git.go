package git

import (
	"fmt"
	"strings"
)

/*
This package contains formatting methods for common Git-hosting webhook events.
Currently supports:
- GitHub
- GitLab

 */

const (
	GITHUB = iota
	GITLAB = iota
	BITBUCKET = iota
)


/*
Push Events

GitHub: https://developer.github.com/v3/activity/events/types/#pushevent
GitLab: https://docs.gitlab.com/ee/user/project/integrations/webhooks.html#push-events

 */

func FormatPushMsg(username string, repo string, branch string, numCommits int, messages []string, commitURL string) (res string) {
	res = fmt.Sprintf("%s pushed %d commit", username, numCommits)
	if numCommits != 1 {
		res += "s"
	}
	res += fmt.Sprintf(" to %s/%s:\n", repo, branch)
	for _, msg := range messages {
		res += fmt.Sprintf("- `%s`\n", formatCommitString(msg, 50))
	}

	urlSplit := strings.Split(commitURL, "://")
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

/*
Issue Events

GitHub: https://developer.github.com/v3/activity/events/types/#issuesevent
Namespace: "opened", "reopened", "closed"

GitLab: https://docs.gitlab.com/ee/user/project/integrations/webhooks.html#issues-events
Namespace: "open", "reopen", "close"

 */

func FormatIssueMsg(action string, username string, repo string, repoNum int, issueTitle string, issueURL string) (res string) {
	switch action {
	case "open", "opened":
		res = fmt.Sprintf("%s opened issue #%d on %s: “%s”\n", username, repoNum, repo, issueTitle)
		res += issueURL
	case "reopen", "reopened":
		res = fmt.Sprintf("%s reopened issue #%d on %s: “%s”\n", username, repoNum, repo, issueTitle)
		res += issueURL
	case "close", "closed":
		res = fmt.Sprintf("%s closed issue #%d on %s.\n", username, repoNum, repo)
		res += issueURL
	}
	return res
}

/*
Pull Request / Merge Request Event

GitHub: https://developer.github.com/v3/activity/events/types/#pullrequestevent
Namespace: "opened", "reopened", closed"

GitLab: https://docs.gitlab.com/ee/user/project/integrations/webhooks.html#merge-request-events
Namespace: "open", "reopen", "close", "merge"

 */

func FormatPullRequestMsg(provider int, action string, username string, repo string, repoNum int, issueTitle string, prURL string, targetBranch string) (res string) {
	var requestName string
	if provider == GITLAB {
		requestName = "merge"
	} else {
		requestName = "pull"
	}

	switch action {
	case "open", "opened":
		res = fmt.Sprintf("%s opened %s request #%d on %s: “%s”\n", username, requestName, repoNum, repo, issueTitle)
		res += prURL
	case "reopen", "reopened":
		res = fmt.Sprintf("%s reopened %s request #%d on %s: “%s”\n", username, requestName, repoNum, repo, issueTitle)
		res += prURL
	case "close", "closed":
		res = fmt.Sprintf("%s closed %s request #%d on %s.\n", username, requestName, repoNum, repo)
		res += prURL
	case "merge", "merged":
		res = fmt.Sprintf("%s merged %s request #%d into %s/%s.\n", username, requestName, repoNum, repo, targetBranch)
		res += prURL
	}

	return res
}
