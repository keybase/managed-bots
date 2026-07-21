package gitlabbot

import (
	"testing"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/stretchr/testify/require"
)

func TestFormatReauthorizationInstructions(t *testing.T) {
	msg := chat1.MsgSummary{ConvID: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	repo := "owner/repo"
	hostedURL := "https://gitlab.example.com"
	httpAddress := "https://bots.example.com"
	secret := "server-secret"

	res := formatReauthorizationInstructions(repo, hostedURL, msg, httpAddress, secret)

	require.Contains(t, res, "https://gitlab.example.com/owner/repo/hooks")
	require.Contains(t, res, "edit the existing webhook")
	require.Contains(t, res, "https://bots.example.com/gitlabbot/webhook")
	require.Contains(t, res, webhookSigningToken(repo, msg.ConvID, secret))
	require.Contains(t, res, "Signing token")
}

func TestFormatSetupInstructionsUsesSigningToken(t *testing.T) {
	msg := chat1.MsgSummary{ConvID: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	res := formatSetupInstructions("owner/repo", "https://gitlab.com", msg, "https://bots.example.com", "server-secret")

	require.Contains(t, res, webhookSigningToken("owner/repo", msg.ConvID, "server-secret"))
	require.Contains(t, res, "Do not configure a “Secret token”")
}

func TestParseRepoInputWithURL(t *testing.T) {
	httpPrefix := "https://mywebsite.com"
	urlRepo := "owner/repo"
	url := httpPrefix + "/" + urlRepo

	hostedURL, repo, err := parseRepoInput(url)
	require.NoError(t, err)

	require.Equal(t, httpPrefix, hostedURL)
	require.Equal(t, repo, urlRepo)
}

func TestParseRepoInputURLSubGroups(t *testing.T) {
	httpPrefix := "https://bobsburgers.com"
	urlRepo := "owner/sub1/sub2/sub3/repo"
	url := httpPrefix + "/" + urlRepo

	hostedURL, repo, err := parseRepoInput(url)
	require.NoError(t, err)

	require.Equal(t, httpPrefix, hostedURL)
	require.Equal(t, repo, urlRepo)
}

func TestParseRepoInputWithNamespace(t *testing.T) {
	httpPrefix := "https://gitlab.com"
	namespace := "own.er/r.e_p-o"

	hostedURL, repo, err := parseRepoInput(namespace)
	require.NoError(t, err)

	require.Equal(t, httpPrefix, hostedURL)
	require.Equal(t, repo, namespace)
}

func TestParseRepoInputNamespaceSubGroups(t *testing.T) {
	httpPrefix := "https://gitlab.com"
	namespace := "owner/sub1/sub2/sub3/repo"

	hostedURL, repo, err := parseRepoInput(namespace)
	require.NoError(t, err)

	require.Equal(t, httpPrefix, hostedURL)
	require.Equal(t, repo, namespace)
}

func TestParseRepoInputWithSuffix(t *testing.T) {
	httpPrefix := "https://gitlab.com"
	namespace := "owner/repo.git"

	hostedURL, repo, err := parseRepoInput(namespace)
	require.NoError(t, err)

	require.Equal(t, httpPrefix, hostedURL)
	require.Equal(t, "owner/repo", repo)
}

func TestParseRepoInputInvalidHostname(t *testing.T) {
	url := "/owner/repo"
	_, _, err := parseRepoInput(url)
	require.Error(t, err)
}

func TestParseRepoInputNoScheme(t *testing.T) {
	// NOTE unfortunately this is a valid gitlab hosted name and self-hosted
	// name.
	url := "mywebsite.com/owner/repo"
	hostedURL, repo, err := parseRepoInput(url)
	require.NoError(t, err)
	require.Equal(t, "https://gitlab.com", hostedURL)
	require.Equal(t, url, repo)
}

func TestParseRepoInputInvalidScheme(t *testing.T) {
	url := "https:mywebsite.com/owner/repo"
	_, _, err := parseRepoInput(url)
	require.Error(t, err)
}

func TestParseRepoInputInvalidNoHostname(t *testing.T) {
	url := "https://owner/repo"
	_, _, err := parseRepoInput(url)
	require.Error(t, err)
}
