package base

import (
	"fmt"
	"strings"
)

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
