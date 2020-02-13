package gitlabbot

import (
	"testing"
)

func TestParseRepoInputWithURL(t *testing.T) {
	httpPrefix := "https://mywebsite.com"
	urlRepo := "owner/repo"
	url := httpPrefix + "/" + urlRepo

	hostedURL, repo, err := parseRepoInput(url)
	if err != nil {
		t.Error(err)
	}

	if hostedURL != httpPrefix {
		t.Errorf("hostedURL is incorrect, got: %s, want: %s.", hostedURL, httpPrefix)
	}

	if repo != urlRepo {
		t.Errorf("repo is incorrect, got: %s, want: %s.", repo, urlRepo)
	}
}

func TestParseRepoInputWithNamespace(t *testing.T) {
	httpPrefix := "https://gitlab.com"
	namespace := "owner/repo"

	hostedURL, repo, err := parseRepoInput(namespace)
	if err != nil {
		t.Error(err)
	}

	if hostedURL != httpPrefix {
		t.Errorf("hostedURL is incorrect, got: %s, want: %s.", hostedURL, httpPrefix)
	}

	if repo != namespace {
		t.Errorf("repo is incorrect, got: %s, want: %s.", repo, namespace)
	}
}

func TestParseRepoInputInvalidHostname(t *testing.T) {
	url := "/owner/repo"

	hostedURL, repo, err := parseRepoInput(url)
	if err == nil {
		t.Errorf("expected error on invalid hostname, got: %s %s", hostedURL, repo)
	}
}

func TestParseRepoInputInvalidNoScheme(t *testing.T) {
	httpPrefix := "mywebsite.com"
	urlRepo := "owner/repo"
	url := httpPrefix + "/" + urlRepo

	_, _, err := parseRepoInput(url)
	if err == nil {
		t.Error(err)
	}
}

func TestParseRepoInputInvalidScheme(t *testing.T) {
	httpPrefix := "https:mywebsite.com"
	urlRepo := "owner/repo"
	url := httpPrefix + "/" + urlRepo

	_, _, err := parseRepoInput(url)
	if err == nil {
		t.Error(err)
	}
}

func TestParseRepoInputInvalidNoHostname(t *testing.T) {
	httpPrefix := "https://"
	urlRepo := "owner/repo"
	url := httpPrefix + "/" + urlRepo

	_, _, err := parseRepoInput(url)
	if err == nil {
		t.Error(err)
	}
}
