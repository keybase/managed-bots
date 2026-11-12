package githubbot

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-github/v89/github"
	"github.com/stretchr/testify/require"
)

func TestRepoFullNameMatches(t *testing.T) {
	repo := &github.Repository{FullName: github.Ptr("Keybase/Managed-Bots")}
	require.Equal(t, true, repoFullNameMatches(repo, "keybase/managed-bots"), "expected case-insensitive match")
	require.Equal(t, false, repoFullNameMatches(repo, "keybase/other"), "expected mismatch for a different repo")
	require.Equal(t, false, repoFullNameMatches(&github.Repository{}, ""), "empty full name must not match an empty repo")
	require.Equal(t, false, repoFullNameMatches(nil, "keybase/managed-bots"), "nil repository must not match")
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
	require.NoError(t, err, "valid small payload rejected")
	require.Equal(t, body, payload)

	big := make([]byte, maxWebhookBodyBytes+1)
	_, err = validateWebhook(httptest.NewRecorder(), newReq(big), secret)
	var tooLarge *http.MaxBytesError
	require.ErrorAs(t, err, &tooLarge, "expected MaxBytesError for oversized body")
}
