package gitlabbot

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
)

const webhookTimestampTolerance = 5 * time.Minute

type webhookAuthenticationMethod int

const (
	webhookAuthenticationInvalid webhookAuthenticationMethod = iota
	webhookAuthenticationLegacyToken
	webhookAuthenticationSignature
)

// webhookSigningKey derives a distinct 32-byte signing key for each
// repository subscription. The master secret is never shared with GitLab.
func webhookSigningKey(repo string, convID chat1.ConvIDStr, masterSecret string) []byte {
	mac := hmac.New(sha256.New, []byte(masterSecret))
	_, _ = mac.Write([]byte("gitlabbot-webhook-signing-v1\x00"))
	_, _ = mac.Write([]byte(repo))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(base.ShortConvID(convID)))
	return mac.Sum(nil)
}

func webhookSigningToken(repo string, convID chat1.ConvIDStr, masterSecret string) string {
	return "whsec_" + base64.StdEncoding.EncodeToString(webhookSigningKey(repo, convID, masterSecret))
}

// legacyWebhookSecretToken preserves the token format used by existing
// X-Gitlab-Token webhooks while new and reauthorized hooks move to signatures.
func legacyWebhookSecretToken(repo string, convID chat1.ConvIDStr, masterSecret string) string {
	digest := sha256.Sum256([]byte(repo + string(base.ShortConvID(convID)) + masterSecret))
	return hex.EncodeToString(digest[:])
}

func authenticateWebhook(
	signingKey []byte,
	expectedLegacyToken string,
	receivedLegacyToken string,
	webhookID string,
	timestamp string,
	signatures string,
	payload []byte,
	now time.Time,
) (webhookAuthenticationMethod, error) {
	if signatures != "" {
		if err := verifyWebhookSignature(signingKey, webhookID, timestamp, signatures, payload, now); err != nil {
			return webhookAuthenticationInvalid, err
		}
		return webhookAuthenticationSignature, nil
	}

	if hmac.Equal([]byte(receivedLegacyToken), []byte(expectedLegacyToken)) {
		return webhookAuthenticationLegacyToken, nil
	}
	return webhookAuthenticationInvalid, fmt.Errorf("legacy webhook token mismatch")
}

func webhookAuthenticationAllowed(
	authMethod webhookAuthenticationMethod, reauthorizationNeeded bool,
) bool {
	switch authMethod {
	case webhookAuthenticationSignature:
		return true
	case webhookAuthenticationLegacyToken:
		return reauthorizationNeeded
	default:
		return false
	}
}

func verifyWebhookSignature(
	signingKey []byte,
	webhookID string,
	timestamp string,
	signatures string,
	payload []byte,
	now time.Time,
) error {
	if webhookID == "" || timestamp == "" || signatures == "" {
		return fmt.Errorf("missing webhook signing headers")
	}

	unixTimestamp, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid webhook timestamp")
	}
	signedAt := time.Unix(unixTimestamp, 0)
	if signedAt.Before(now.Add(-webhookTimestampTolerance)) || signedAt.After(now.Add(webhookTimestampTolerance)) {
		return fmt.Errorf("webhook timestamp outside allowed window")
	}

	mac := hmac.New(sha256.New, signingKey)
	_, _ = mac.Write([]byte(webhookID))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(payload)
	expected := []byte("v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil)))

	for _, signature := range strings.Fields(signatures) {
		if hmac.Equal([]byte(signature), expected) {
			return nil
		}
	}
	return fmt.Errorf("webhook signature mismatch")
}
