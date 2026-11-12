package githubbot

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-github/v89/github"
)

func TestRepoFullNameMatches(t *testing.T) {
	repo := &github.Repository{FullName: github.Ptr("Keybase/Managed-Bots")}
	if !repoFullNameMatches(repo, "keybase/managed-bots") {
		t.Error("expected case-insensitive match")
	}
	if repoFullNameMatches(repo, "keybase/other") {
		t.Error("expected mismatch for a different repo")
	}
	if repoFullNameMatches(&github.Repository{}, "") {
		t.Error("empty full name must not match an empty repo")
	}
	if repoFullNameMatches(nil, "keybase/managed-bots") {
		t.Error("nil repository must not match")
	}
}

func TestFormatCheckRunSanitizesExternalText(t *testing.T) {
	evt := &github.CheckRunEvent{
		Action: github.Ptr("completed"),
		Repo:   &github.Repository{Name: github.Ptr("repo")},
		CheckRun: &github.CheckRun{
			Name:       github.Ptr("@channel build `x`"),
			Conclusion: github.Ptr("success"),
			CheckSuite: &github.CheckSuite{HeadBranch: github.Ptr("@here\n!evil")},
		},
	}
	res := formatCheckRunMessage(evt, "")
	for _, bad := range []string{"@channel", "@here", "`", "\n!evil"} {
		if strings.Contains(res, bad) {
			t.Errorf("message %q contains unsanitized %q", res, bad)
		}
	}
}

func TestValidateWebhookLimitsBody(t *testing.T) {
	const secret = "s3cret"
	body := []byte(`{"zen":"ok"}`)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	newReq := func(body []byte) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/githubbot/webhook", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Hub-Signature-256", sig)
		return req
	}

	payload, err := validateWebhook(httptest.NewRecorder(), newReq(body), secret)
	if err != nil {
		t.Fatalf("valid small payload rejected: %s", err)
	}
	if !bytes.Equal(payload, body) {
		t.Fatalf("payload mismatch: %q", payload)
	}

	big := make([]byte, maxWebhookBodyBytes+1)
	_, err = validateWebhook(httptest.NewRecorder(), newReq(big), secret)
	var tooLarge *http.MaxBytesError
	if !errors.As(err, &tooLarge) {
		t.Fatalf("expected MaxBytesError for oversized body, got %v", err)
	}
}
