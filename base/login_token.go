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

// loginTokenMAC computes the HMAC over purpose, message and timestamp. The
// purpose is a per-bot prefix so a token minted by one bot can't be replayed
// against another bot that shares the same secret.
func loginTokenMAC(secret, purpose, message, tsHex string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(purpose + "\x00" + message + ":" + tsHex))
	return mac.Sum(nil)
}

// MakeLoginToken returns a time-stamped HMAC-SHA256 token bound to purpose and message.
func MakeLoginToken(secret, purpose, message string) string {
	tsHex := strconv.FormatInt(time.Now().Unix(), 16)
	return tsHex + "." + hex.EncodeToString(loginTokenMAC(secret, purpose, message, tsHex))
}

// VerifyLoginToken checks the token's HMAC and age against LoginTokenMaxAge.
func VerifyLoginToken(secret, purpose, message, token string) bool {
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
	submittedMAC, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}
	return hmac.Equal(submittedMAC, loginTokenMAC(secret, purpose, message, tsHex))
}
