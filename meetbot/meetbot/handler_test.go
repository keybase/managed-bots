package meetbot

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
)

func TestIsTokenRejected(t *testing.T) {
	require.True(t, isTokenRejected(&googleapi.Error{Code: 401}))
	require.True(t, isTokenRejected(fmt.Errorf("calendar: %w", &googleapi.Error{Code: 401})))
	require.True(t, isTokenRejected(&oauth2.RetrieveError{ErrorCode: "invalid_grant"}))

	require.False(t, isTokenRejected(&googleapi.Error{Code: 500}))
	require.False(t, isTokenRejected(&googleapi.Error{Code: 429}))
	require.False(t, isTokenRejected(&googleapi.Error{Code: 403, Message: "accessNotConfigured"}))
	require.False(t, isTokenRejected(fmt.Errorf("calendar: %w", &googleapi.Error{Code: 503})))
	require.False(t, isTokenRejected(errors.New("connection reset")))
}
