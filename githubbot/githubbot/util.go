package githubbot

import (
	"fmt"
	"strings"

	"github.com/google/go-github/v28/github"
)

func formatPushMsg(evt *github.PushEvent) (res string) {
	// refs are always given in the form "refs/heads/{branch name}" or "refs/tags/{tag name}"
	branch := strings.Split(evt.GetRef(), "refs/")[1]
	if strings.HasPrefix(branch, "heads/") {
		branch = strings.Split(branch, "heads/")[1]
	}

	// if we got a tag ref, just leave it as "tags/{tag name}"

	res = fmt.Sprintf("%s pushed %d commit", evt.GetPusher().GetName(), len(evt.Commits))
	if len(evt.Commits) != 1 {
		res += "s"
	}
	res += fmt.Sprintf(" to %s/%s.\n%s", evt.GetRepo().GetName(), branch, evt.GetCompare())
	return res
}

func formatIssueMsg(evt *github.IssuesEvent) (res string) {
	action := evt.Action
	if action != nil {
		switch *action {
		case "opened":
			res = fmt.Sprintf("%s opened issue #%d on %s: \"%s\"\n", evt.GetSender().GetLogin(), evt.GetIssue().GetNumber(), evt.GetRepo().GetName(), evt.GetIssue().GetTitle())
		case "closed":
			res = fmt.Sprintf("%s closed issue #%d on %s.\n", evt.GetSender().GetLogin(), evt.GetIssue().GetNumber(), evt.GetRepo().GetName())
		}
	}
	res += evt.GetIssue().GetHTMLURL()
	return res
}

func formatPRMsg(evt *github.PullRequestEvent) (res string) {
	action := evt.Action
	if action != nil {
		switch *action {
		case "opened":
			res = fmt.Sprintf("%s opened pull request #%d on %s.\n", evt.GetSender().GetLogin(), evt.GetNumber(), evt.GetRepo().GetName())
		case "closed":
			if evt.GetPullRequest().GetMerged() {
				// PR was merged
				res = fmt.Sprintf("%s merged pull request #%d into %s/%s.\n", evt.GetPullRequest().GetMergedBy().GetLogin(), evt.GetNumber(), evt.GetRepo().GetName(), evt.GetPullRequest().GetBase().GetRef())
			} else {
				// PR was closed without merging
				res = fmt.Sprintf("%s closed pull request #%d on %s.\n", evt.GetSender().GetLogin(), evt.GetNumber(), evt.GetRepo().GetName())
			}
		}
	}
	res += evt.GetPullRequest().GetHTMLURL()
	return res
}

func shortConvID(convID string) string {
	if len(convID) <= 20 {
		return convID
	}
	return convID[:20]
}
