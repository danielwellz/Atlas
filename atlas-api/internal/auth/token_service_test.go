package auth_test

import (
	"testing"
	"time"

	"github.com/atlas/atlas-api/internal/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestIssueAndValidateAccessToken(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.February, 26, 12, 0, 0, 0, time.UTC)
	service := auth.NewTokenService("test-secret", 15*time.Minute, 24*time.Hour, func() time.Time {
		return now
	})

	userID := uuid.New()
	sessionID := uuid.New()
	token, err := service.IssueAccessToken(userID, sessionID)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	claims, err := service.ValidateAccessToken(token)
	require.NoError(t, err)
	require.Equal(t, userID.String(), claims.UserID)
	require.Equal(t, sessionID.String(), claims.SessionID)
	require.Equal(t, userID.String(), claims.Subject)
}

func TestValidateAccessTokenExpired(t *testing.T) {
	t.Parallel()

	currentTime := time.Date(2026, time.February, 26, 12, 0, 0, 0, time.UTC)
	service := auth.NewTokenService("test-secret", time.Minute, 24*time.Hour, func() time.Time {
		return currentTime
	})

	token, err := service.IssueAccessToken(uuid.New(), uuid.New())
	require.NoError(t, err)

	currentTime = currentTime.Add(2 * time.Minute)
	_, err = service.ValidateAccessToken(token)
	require.ErrorIs(t, err, auth.ErrExpiredToken)
}

func TestValidateAccessTokenInvalidSignature(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.February, 26, 12, 0, 0, 0, time.UTC)
	issuer := auth.NewTokenService("issuer-secret", 15*time.Minute, 24*time.Hour, func() time.Time {
		return now
	})
	validator := auth.NewTokenService("validator-secret", 15*time.Minute, 24*time.Hour, func() time.Time {
		return now
	})

	token, err := issuer.IssueAccessToken(uuid.New(), uuid.New())
	require.NoError(t, err)

	_, err = validator.ValidateAccessToken(token)
	require.ErrorIs(t, err, auth.ErrInvalidToken)
}

func TestNewRefreshToken(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.February, 26, 12, 0, 0, 0, time.UTC)
	service := auth.NewTokenService("test-secret", 15*time.Minute, 7*24*time.Hour, func() time.Time {
		return now
	})

	plain, hashed, expiresAt, err := service.NewRefreshToken()
	require.NoError(t, err)
	require.NotEmpty(t, plain)
	require.NotEmpty(t, hashed)
	require.Equal(t, auth.HashToken(plain), hashed)
	require.Equal(t, now.Add(7*24*time.Hour), expiresAt)
}
