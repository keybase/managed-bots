package githubbot

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v28/github"
)

func makeSecret(repo string, secret string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(repo+secret)))
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

func formatSetupInstructions(repo string, httpAddress string, secret string) (res string) {
	back := "`"
	message := fmt.Sprintf(`
To configure your repository to send notifications, go to https://github.com/%s/settings/hooks and add a new webhook.
For "Payload URL", enter %s%s/githubbot/webhook%s.
Set "Content Type" to %sapplication/json%s.
For "Secret", enter %s%s%s.

Happy coding!`,
		repo, back, httpAddress, back, back, back, back, makeSecret(repo, secret), back)
	return message
}

func formatPushMsg(evt *github.PushEvent) (res string) {
	branch := refToName(evt.GetRef())

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
			res += evt.GetIssue().GetHTMLURL()
			break
		case "closed":
			res = fmt.Sprintf("%s closed issue #%d on %s.\n", evt.GetSender().GetLogin(), evt.GetIssue().GetNumber(), evt.GetRepo().GetName())
			res += evt.GetIssue().GetHTMLURL()
			break
		}
	}
	return res
}

func formatPRMsg(evt *github.PullRequestEvent) (res string) {
	action := evt.Action
	if action != nil {
		switch *action {
		case "opened":
			res = fmt.Sprintf("%s opened pull request #%d on %s.\n", evt.GetSender().GetLogin(), evt.GetNumber(), evt.GetRepo().GetName())
			res += evt.GetPullRequest().GetHTMLURL()
			break
		case "closed":
			if evt.GetPullRequest().GetMerged() {
				// PR was merged
				res = fmt.Sprintf("%s merged pull request #%d into %s/%s.\n", evt.GetPullRequest().GetMergedBy().GetLogin(), evt.GetNumber(), evt.GetRepo().GetName(), evt.GetPullRequest().GetBase().GetRef())
				res += evt.GetPullRequest().GetHTMLURL()
			} else {
				// PR was closed without merging
				res = fmt.Sprintf("%s closed pull request #%d on %s.\n", evt.GetSender().GetLogin(), evt.GetNumber(), evt.GetRepo().GetName())
				res += evt.GetPullRequest().GetHTMLURL()
			}
			break
		}
	}
	return res
}

func formatCheckSuiteMsg(evt *github.CheckSuiteEvent) (res string) {
	action := evt.Action
	if *action == "completed" {
		suite := evt.GetCheckSuite()
		repo := evt.GetRepo().GetName()
		isPullRequest := len(suite.PullRequests) > 0
		switch suite.GetConclusion() {
		case "success":
			if !isPullRequest {
				res = fmt.Sprintf("Tests passed for %s/%s.", repo, suite.GetHeadBranch())
				break
			}
			// TODO: mention PR author when tests pass?
			pr := suite.PullRequests[0]
			res = fmt.Sprintf("All tests passed for pull request #%d on %s.\n%s", pr.GetNumber(), repo, pr.GetHTMLURL())
			break
		case "failure", "timed_out":
			if !isPullRequest {
				res = fmt.Sprintf("Tests failed for %s/%s.", repo, suite.GetHeadBranch())
				break
			}
			// TODO: mention PR author when tests fail?
			pr := suite.PullRequests[0]
			res = fmt.Sprintf("Tests failed for pull request #%d on %s.\n%s", pr.GetNumber(), repo, pr.GetHTMLURL())
			break
		}
	}
	return res
}

func shortConvID(convID string) string {
	if len(convID) <= 20 {
		return convID
	}
	return convID[:20]
}

func getDefaultBranch(repo string) (branch string, err error) {
	client := github.NewClient(nil)
	args := strings.Split(repo, "/")
	if len(args) != 2 {
		return "", fmt.Errorf("getDefaultBranch: invalid repo %s", repo)
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
