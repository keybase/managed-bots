package gitlabbot

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/stretchr/testify/require"
)

func TestWebhookSigningToken(t *testing.T) {
	convID := chat1.ConvIDStr("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	token := webhookSigningToken("owner/repo", convID, "master-secret")

	require.True(t, strings.HasPrefix(token, "whsec_"))
	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(token, "whsec_"))
	require.NoError(t, err)
	require.Len(t, key, sha256.Size)
	require.NotEqual(t, token, webhookSigningToken("owner/other", convID, "master-secret"))
	require.NotEqual(t, token, webhookSigningToken("owner/repo", convID, "other-master-secret"))
}

func TestLegacyWebhookSecretToken(t *testing.T) {
	convID := chat1.ConvIDStr("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")

	require.Equal(
		t,
		"ecd6811c2c34961c3f7d44d315d59e3ab58e0ae5c28527fb198e876ee74a3591",
		legacyWebhookSecretToken("owner/repo", convID, "master-secret"),
	)
}

func TestVerifyWebhookSignature(t *testing.T) {
	now := time.Unix(1_750_000_000, 0)
	timestamp := strconv.FormatInt(now.Unix(), 10)
	payload := []byte(`{"object_kind":"push"}`)
	webhookID := "webhook-id"
	key := []byte("01234567890123456789012345678901")

	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(webhookID + "." + timestamp + "."))
	_, _ = mac.Write(payload)
	signature := "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))

	require.NoError(t, verifyWebhookSignature(key, webhookID, timestamp, signature, payload, now))
	require.NoError(t, verifyWebhookSignature(key, webhookID, timestamp, "v1,invalid "+signature, payload, now))
	require.Error(t, verifyWebhookSignature(key, webhookID, timestamp, signature, append(payload, ' '), now))
	require.Error(t, verifyWebhookSignature(key, webhookID, timestamp, signature, payload, now.Add(webhookTimestampTolerance+time.Second)))
	require.Error(t, verifyWebhookSignature(key, "", timestamp, signature, payload, now))
}

func TestAuthenticateWebhookDoesNotDowngradeInvalidSignature(t *testing.T) {
	now := time.Unix(1_750_000_000, 0)
	payload := []byte(`{"object_kind":"push"}`)
	key := []byte("01234567890123456789012345678901")

	method, err := authenticateWebhook(
		key,
		"valid-legacy-token",
		"valid-legacy-token",
		"webhook-id",
		strconv.FormatInt(now.Unix(), 10),
		"v1,invalid",
		payload,
		now,
	)

	require.Error(t, err)
	require.Equal(t, webhookAuthenticationInvalid, method)
}

func TestAuthenticateWebhookPrefersValidSignature(t *testing.T) {
	now := time.Unix(1_750_000_000, 0)
	timestamp := strconv.FormatInt(now.Unix(), 10)
	payload := []byte(`{"object_kind":"push"}`)
	webhookID := "webhook-id"
	key := []byte("01234567890123456789012345678901")
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(webhookID + "." + timestamp + "."))
	_, _ = mac.Write(payload)
	signature := "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))

	method, err := authenticateWebhook(
		key,
		"valid-legacy-token",
		"wrong-legacy-token",
		webhookID,
		timestamp,
		signature,
		payload,
		now,
	)

	require.NoError(t, err)
	require.Equal(t, webhookAuthenticationSignature, method)
}

func TestAuthenticateWebhookFallsBackWhenSignatureAbsent(t *testing.T) {
	method, err := authenticateWebhook(
		[]byte("unused"),
		"valid-legacy-token",
		"valid-legacy-token",
		"",
		"",
		"",
		nil,
		time.Now(),
	)

	require.NoError(t, err)
	require.Equal(t, webhookAuthenticationLegacyToken, method)
}

func TestWebhookAuthenticationAllowed(t *testing.T) {
	tests := []struct {
		name                  string
		authMethod            webhookAuthenticationMethod
		reauthorizationNeeded bool
		allowed               bool
	}{
		{
			name:                  "signature completes pending reauthorization",
			authMethod:            webhookAuthenticationSignature,
			reauthorizationNeeded: true,
			allowed:               true,
		},
		{
			name:                  "signature remains valid after reauthorization",
			authMethod:            webhookAuthenticationSignature,
			reauthorizationNeeded: false,
			allowed:               true,
		},
		{
			name:                  "legacy token works while reauthorization is pending",
			authMethod:            webhookAuthenticationLegacyToken,
			reauthorizationNeeded: true,
			allowed:               true,
		},
		{
			name:                  "legacy token cannot downgrade signed subscription",
			authMethod:            webhookAuthenticationLegacyToken,
			reauthorizationNeeded: false,
			allowed:               false,
		},
		{
			name:                  "invalid authentication is rejected",
			authMethod:            webhookAuthenticationInvalid,
			reauthorizationNeeded: true,
			allowed:               false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(
				t,
				test.allowed,
				webhookAuthenticationAllowed(test.authMethod, test.reauthorizationNeeded),
			)
		})
	}
}
