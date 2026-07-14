package base

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

const LoginTokenMaxAge = 7 * 24 * time.Hour

// MakeLoginToken returns a time-stamped HMAC-SHA256 token bound to message.
func MakeLoginToken(secret, message string) string {
	tsHex := strconv.FormatInt(time.Now().Unix(), 16)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message + ":" + tsHex))
	return tsHex + "." + hex.EncodeToString(mac.Sum(nil))
}

// VerifyLoginToken checks the token's HMAC and age against LoginTokenMaxAge.
func VerifyLoginToken(secret, message, token string) bool {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return false
	}
	tsHex := parts[0]
	ts, err := strconv.ParseInt(tsHex, 16, 64)
	if err != nil {
		return false
	}
	age := time.Since(time.Unix(ts, 0))
	if age < 0 || age > LoginTokenMaxAge {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message + ":" + tsHex))
	submittedMAC, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}
	return hmac.Equal(submittedMAC, mac.Sum(nil))
}
