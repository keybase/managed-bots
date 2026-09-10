package base

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestShouldRetryAuth(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		require.False(t, ShouldRetryAuth(nil))
	})

	t.Run("invalid_grant RetrieveError", func(t *testing.T) {
		err := &oauth2.RetrieveError{ErrorCode: "invalid_grant", Body: []byte(`{"error":"invalid_grant"}`)}
		require.True(t, ShouldRetryAuth(err))
		require.True(t, ShouldRetryAuth(fmt.Errorf("unable to renew token: %w", err)))
	})

	t.Run("invalid_grant in body only", func(t *testing.T) {
		err := &oauth2.RetrieveError{Body: []byte(`{"error":"invalid_grant"}`)}
		require.True(t, ShouldRetryAuth(err))
	})

	t.Run("missing refresh token", func(t *testing.T) {
		require.True(t, ShouldRetryAuth(errors.New("oauth2: token expired and refresh token is not set")))
	})

	t.Run("transient cannot fetch token", func(t *testing.T) {
		err := &oauth2.RetrieveError{Body: []byte("connection reset"), ErrorCode: ""}
		require.False(t, ShouldRetryAuth(err))
		require.False(t, ShouldRetryAuth(errors.New("oauth2: cannot fetch token: 500 Internal Server Error")))
	})

	t.Run("unrelated", func(t *testing.T) {
		require.False(t, ShouldRetryAuth(errors.New("calendar: 404 not found")))
	})
}

type stubTokenSource struct {
	tok *oauth2.Token
	err error
}

func (s stubTokenSource) Token() (*oauth2.Token, error) {
	return s.tok, s.err
}

func TestPersistTokenSource(t *testing.T) {
	t.Parallel()
	baseTok := oauth2.Token{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		Expiry:       time.Now().Add(time.Hour),
	}

	t.Run("no write when unchanged", func(t *testing.T) {
		orig := baseTok
		var puts int
		src := PersistTokenSource(context.Background(), &orig, stubTokenSource{tok: &orig},
			func(context.Context, *oauth2.Token) error {
				puts++
				return nil
			})
		tok, err := src.Token()
		require.NoError(t, err)
		require.Equal(t, "old-access", tok.AccessToken)
		require.Equal(t, 0, puts)
	})

	t.Run("writes on refresh token rotation", func(t *testing.T) {
		stored := baseTok
		rotated := &oauth2.Token{
			AccessToken:  "new-access",
			RefreshToken: "new-refresh",
			Expiry:       time.Now().Add(2 * time.Hour),
		}
		var got *oauth2.Token
		src := PersistTokenSource(context.Background(), &stored, stubTokenSource{tok: rotated},
			func(_ context.Context, tok *oauth2.Token) error {
				got = tok
				return nil
			})
		tok, err := src.Token()
		require.NoError(t, err)
		require.Equal(t, "new-refresh", tok.RefreshToken)
		require.Equal(t, "new-refresh", stored.RefreshToken)
		require.Equal(t, "new-refresh", got.RefreshToken)
	})

	t.Run("put uses uncancelled context", func(t *testing.T) {
		orig := baseTok
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		rotated := &oauth2.Token{
			AccessToken:  "new-access",
			RefreshToken: "new-refresh",
			Expiry:       time.Now().Add(time.Hour),
		}
		src := PersistTokenSource(ctx, &orig, stubTokenSource{tok: rotated},
			func(putCtx context.Context, _ *oauth2.Token) error {
				require.NoError(t, putCtx.Err())
				return nil
			})
		_, err := src.Token()
		require.NoError(t, err)
	})

	t.Run("put error", func(t *testing.T) {
		orig := baseTok
		rotated := &oauth2.Token{
			AccessToken:  "new-access",
			RefreshToken: "old-refresh",
			Expiry:       time.Now().Add(time.Hour),
		}
		src := PersistTokenSource(context.Background(), &orig, stubTokenSource{tok: rotated},
			func(context.Context, *oauth2.Token) error {
				return errors.New("db down")
			})
		_, err := src.Token()
		require.ErrorContains(t, err, "unable to update token")
		require.ErrorContains(t, err, "db down")
	})
}

func TestConfigTokenSourceZeroExpiry(t *testing.T) {
	t.Parallel()
	token := &oauth2.Token{AccessToken: "a", RefreshToken: "r"}
	require.True(t, token.Valid(), "zero expiry is Valid() in oauth2")

	//nolint:gosec // G101: False positive - TokenURL is a dummy loopback address, not credentials
	cfg := &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: "http://127.0.0.1:1"}}
	src := ConfigTokenSource(context.Background(), cfg, token)
	_, err := src.Token()
	require.Error(t, err, "zero-expiry token with refresh token must be refreshed, not reused")
}

func TestPersistTokenSourceConcurrent(t *testing.T) {
	t.Parallel()
	token := &oauth2.Token{AccessToken: "a", RefreshToken: "r", Expiry: time.Now().Add(time.Hour)}
	rotated := &oauth2.Token{AccessToken: "b", RefreshToken: "r2", Expiry: time.Now().Add(2 * time.Hour)}
	src := PersistTokenSource(context.Background(), token, stubTokenSource{tok: rotated},
		func(context.Context, *oauth2.Token) error { return nil })
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			_, err := src.Token()
			require.NoError(t, err)
		})
	}
	wg.Wait()
}
