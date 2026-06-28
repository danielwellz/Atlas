package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/atlas/atlas-api/internal/auth"
	"github.com/atlas/atlas-api/internal/httpapi/middleware"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type tokenValidatorStub struct {
	claims auth.AccessTokenClaims
	err    error
}

func (s tokenValidatorStub) ValidateAccessToken(_ string) (auth.AccessTokenClaims, error) {
	if s.err != nil {
		return auth.AccessTokenClaims{}, s.err
	}
	return s.claims, nil
}

func TestAuthenticatorAllowsPublicAllowlistedPath(t *testing.T) {
	t.Parallel()

	publicAllowlist := map[string]struct{}{"/api/v1/health": {}}
	mw := middleware.Authenticator(zap.NewNop(), tokenValidatorStub{}, publicAllowlist)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	nextCalled := false
	handler := mw(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	}))
	handler.ServeHTTP(rec, req)

	require.True(t, nextCalled)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthenticatorInjectsOptionalUserIDForPublicAllowlistedPath(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	publicAllowlist := map[string]struct{}{"/api/v1/events": {}}
	mw := middleware.Authenticator(zap.NewNop(), tokenValidatorStub{claims: auth.AccessTokenClaims{UserID: userID.String(), SessionID: uuid.New().String()}}, publicAllowlist)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/events", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()

	nextCalled := false
	handler := mw(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		nextCalled = true
		ctxUserID, ok := middleware.AuthenticatedUserID(r.Context())
		require.True(t, ok)
		require.Equal(t, userID, ctxUserID)
	}))
	handler.ServeHTTP(rec, req)

	require.True(t, nextCalled)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthenticatorBlocksNonAllowlistedV1PathWithoutToken(t *testing.T) {
	t.Parallel()

	publicAllowlist := map[string]struct{}{"/api/v1/health": {}}
	mw := middleware.Authenticator(zap.NewNop(), tokenValidatorStub{}, publicAllowlist)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	rec := httptest.NewRecorder()

	nextCalled := false
	handler := mw(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	}))
	handler.ServeHTTP(rec, req)

	require.False(t, nextCalled)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthenticatorInjectsUserID(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	publicAllowlist := map[string]struct{}{"/api/v1/health": {}}
	mw := middleware.Authenticator(zap.NewNop(), tokenValidatorStub{claims: auth.AccessTokenClaims{UserID: userID.String(), SessionID: uuid.New().String()}}, publicAllowlist)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxUserID, ok := middleware.AuthenticatedUserID(r.Context())
		require.True(t, ok)
		require.Equal(t, userID, ctxUserID)
		w.WriteHeader(http.StatusNoContent)
	}))
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestAuthenticatorAllowsWildcardAllowlistedPath(t *testing.T) {
	t.Parallel()

	publicAllowlist := map[string]struct{}{"/api/v1/exercises/*": {}}
	mw := middleware.Authenticator(zap.NewNop(), tokenValidatorStub{}, publicAllowlist)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/exercises/123", nil)
	rec := httptest.NewRecorder()

	nextCalled := false
	handler := mw(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	}))
	handler.ServeHTTP(rec, req)

	require.True(t, nextCalled)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthenticatorBlocksNonAllowlistedWildcardPath(t *testing.T) {
	t.Parallel()

	publicAllowlist := map[string]struct{}{"/api/v1/exercises/*": {}}
	mw := middleware.Authenticator(zap.NewNop(), tokenValidatorStub{}, publicAllowlist)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workouts/history", nil)
	rec := httptest.NewRecorder()

	nextCalled := false
	handler := mw(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	}))
	handler.ServeHTTP(rec, req)

	require.False(t, nextCalled)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthenticatorAllowsOptionsPreflight(t *testing.T) {
	t.Parallel()

	publicAllowlist := map[string]struct{}{"/api/v1/health": {}}
	mw := middleware.Authenticator(zap.NewNop(), tokenValidatorStub{}, publicAllowlist)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/me", nil)
	rec := httptest.NewRecorder()

	nextCalled := false
	handler := mw(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	}))
	handler.ServeHTTP(rec, req)

	require.True(t, nextCalled)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthenticatorAllowsNonV1Path(t *testing.T) {
	t.Parallel()

	publicAllowlist := map[string]struct{}{"/api/v1/health": {}}
	mw := middleware.Authenticator(zap.NewNop(), tokenValidatorStub{}, publicAllowlist)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	nextCalled := false
	handler := mw(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	}))
	handler.ServeHTTP(rec, req)

	require.True(t, nextCalled)
	require.Equal(t, http.StatusOK, rec.Code)
}
