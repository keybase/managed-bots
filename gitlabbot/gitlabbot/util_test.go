package gitlabbot

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestParseRepoInputWithURL(t *testing.T) {
	httpPrefix := "https://mywebsite.com"
	urlRepo := "owner/repo"
	url := httpPrefix + "/" + urlRepo

	hostedURL, repo, err := parseRepoInput(url)
	require.NoError(t, err)

	require.Equal(t, httpPrefix, hostedURL)
	require.Equal(t, repo, urlRepo)
}

func TestParseRepoInputWithNamespace(t *testing.T) {
	httpPrefix := "https://gitlab.com"
	namespace := "owner/repo"

	hostedURL, repo, err := parseRepoInput(namespace)
	require.NoError(t, err)

	require.Equal(t, httpPrefix, hostedURL)
	require.Equal(t, repo, namespace)
}

func TestParseRepoInputInvalidHostname(t *testing.T) {
	url := "/owner/repo"
	_, _, err := parseRepoInput(url)
	require.Error(t, err)
}

func TestParseRepoInputInvalidNoScheme(t *testing.T) {
	url := "mywebsite.com/owner/repo"
	_, _, err := parseRepoInput(url)
	require.Error(t, err)
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
