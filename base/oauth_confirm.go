package base

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"golang.org/x/oauth2"
)

// The chat side of linking an account: the requester confirms a pending link
// (see OAuthFlow) by sending "<confirmCmd> <code>".

// An OAuth link has to be started, authorized at the provider, and confirmed
// from Keybase within OAuthStateTTL of being requested.
const (
	OAuthStateTTL        = 15 * time.Minute
	OAuthStateTTLSeconds = int64(OAuthStateTTL / time.Second)
)

// MaxConfirmAttempts is how many wrong confirmation codes a requester may
// send before their pending links stop accepting codes.
const MaxConfirmAttempts = 5

// confirmCodeAlphabet has 32 characters (no 0/O or 1/I), so each random byte
// maps to it without bias via b%32.
const confirmCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

const confirmCodeLen = 8

// MakeConfirmCode returns a random code formatted for display (e.g.
// "K7QM-2XRP") and the hash to store for it.
func MakeConfirmCode() (code, codeHash string, err error) {
	b, err := RandBytes(confirmCodeLen)
	if err != nil {
		return "", "", err
	}
	raw := make([]byte, confirmCodeLen)
	for i := range b {
		raw[i] = confirmCodeAlphabet[int(b[i])%len(confirmCodeAlphabet)]
	}
	code = string(raw[:confirmCodeLen/2]) + "-" + string(raw[confirmCodeLen/2:])
	return code, HashConfirmCode(code), nil
}

// HashConfirmCode normalizes a user-entered code (case, dashes, spaces) and
// returns its hex SHA-256.
func HashConfirmCode(code string) string {
	code = strings.ToUpper(code)
	code = strings.NewReplacer("-", "", " ", "").Replace(code)
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

// PendingOAuthCode is the confirmation code hash of an OAuth link that
// finished at the provider and is waiting for the requester to confirm it.
type PendingOAuthCode struct {
	State    string
	CodeHash string
}

// PendingOAuthStorage is implemented by OAuthStateDB.
type PendingOAuthStorage interface {
	// GetPendingCodes returns the requester's unexpired links that are
	// awaiting confirmation and have fewer than MaxConfirmAttempts failures.
	GetPendingCodes(ctx context.Context, requester string) ([]PendingOAuthCode, error)
	GetPendingToken(ctx context.Context, state string) (*oauth2.Token, error)
	IncrementConfirmAttempts(ctx context.Context, requester string) error
	CompleteState(ctx context.Context, state string) (bool, error)
}

// MatchConfirmCode returns the state of the requester's pending link matching
// code, or "" if none matches. A miss counts as a failed attempt against every
// pending link of the requester, so the code can't be brute forced.
func MatchConfirmCode(ctx context.Context, storage PendingOAuthStorage, requester, code string) (string, error) {
	pending, err := storage.GetPendingCodes(ctx, requester)
	if err != nil || len(pending) == 0 {
		return "", err
	}
	codeHash := []byte(HashConfirmCode(code))
	for _, p := range pending {
		if subtle.ConstantTimeCompare(codeHash, []byte(p.CodeHash)) == 1 {
			return p.State, nil
		}
	}
	return "", storage.IncrementConfirmAttempts(ctx, requester)
}

// ConfirmPendingOAuth checks code against the requester's pending links. On a
// match it passes the token to save and only then marks the link used, so if
// save fails the requester can send the same code again. It returns false if
// nothing was confirmed: no match, the link expired, or a concurrent
// confirmation of the same link won.
func ConfirmPendingOAuth(
	ctx context.Context,
	storage PendingOAuthStorage,
	requester, code string,
	save func(ctx context.Context, state string, token *oauth2.Token) error,
) (bool, error) {
	state, err := MatchConfirmCode(ctx, storage, requester, code)
	if err != nil || state == "" {
		return false, err
	}
	token, err := storage.GetPendingToken(ctx, state)
	if err != nil || token == nil {
		return false, err
	}
	if err := save(ctx, state, token); err != nil {
		return false, err
	}
	return storage.CompleteState(ctx, state)
}

// ParseConfirmCmd reports whether body is confirmCmd (e.g. "!github confirm"),
// and returns the code argument, which is empty if missing. A code typed with
// spaces ("K7QM 2XRP") is returned as typed; HashConfirmCode ignores them.
func ParseConfirmCmd(body, confirmCmd string) (code string, isConfirm bool) {
	fields := strings.Fields(body)
	cmdFields := strings.Fields(confirmCmd)
	if len(fields) < len(cmdFields) {
		return "", false
	}
	for i := range cmdFields {
		if !strings.EqualFold(fields[i], cmdFields[i]) {
			return "", false
		}
	}
	return strings.Join(fields[len(cmdFields):], " "), true
}

// Chat replies for confirmations; retryCmd tells the user how to get a new
// link.

func ConfirmUsageMsg(confirmCmd string) string {
	return fmt.Sprintf("Send `%s <code>` with the code shown after you authorized me.", confirmCmd)
}

func ConfirmNoMatchMsg(retryCmd string) string {
	return fmt.Sprintf("That code doesn't match an authorization you started in the last %d minutes, or it expired. %s",
		int(OAuthStateTTL/time.Minute), retryCmd)
}

const ConfirmSaveFailedMsg = "Something went wrong saving your authorization. Send the same code again to retry."

// ConfirmCommandAdvertisement advertises confirmCmd (without the leading "!").
// Restricted bots only receive advertised commands, so it must be listed.
func ConfirmCommandAdvertisement(confirmCmd string) chat1.UserBotCommandInput {
	return chat1.UserBotCommandInput{
		Name:        strings.TrimPrefix(confirmCmd, "!"),
		Description: "Finish connecting an account with the code shown after you authorize me",
		Usage:       "<code>",
	}
}

func MarshalPendingToken(token *oauth2.Token) (string, error) {
	//nolint:gosec // G117: Intentional - the pending token is stored in the DB alongside confirmed tokens, never logged or sent
	dat, err := json.Marshal(token)
	if err != nil {
		return "", err
	}
	return string(dat), nil
}

func UnmarshalPendingToken(dat string) (*oauth2.Token, error) {
	var token oauth2.Token
	if err := json.Unmarshal([]byte(dat), &token); err != nil {
		return nil, fmt.Errorf("invalid pending token: %w", err)
	}
	return &token, nil
}
