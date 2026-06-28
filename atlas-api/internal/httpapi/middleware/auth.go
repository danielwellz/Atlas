package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/atlas/atlas-api/internal/auth"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AccessTokenValidator interface {
	ValidateAccessToken(tokenString string) (auth.AccessTokenClaims, error)
}

func Authenticator(logger *zap.Logger, validator AccessTokenValidator, publicPathAllowlist map[string]struct{}) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions || !pathRequiresAuth(r.URL.Path, publicPathAllowlist) {
				next.ServeHTTP(w, withOptionalAuthContext(r, logger, validator))
				return
			}

			token, ok := bearerToken(r.Header.Get("Authorization"))
			if !ok {
				writeUnauthorized(w, "missing or invalid authorization header")
				return
			}

			claims, err := validator.ValidateAccessToken(token)
			if err != nil {
				if errors.Is(err, auth.ErrExpiredToken) {
					writeUnauthorized(w, "access token expired")
					return
				}
				logger.Debug("access token validation failed", zap.Error(err))
				writeUnauthorized(w, "invalid access token")
				return
			}

			userID, err := uuid.Parse(claims.UserID)
			if err != nil {
				writeUnauthorized(w, "invalid access token")
				return
			}

			next.ServeHTTP(w, r.WithContext(SetAuthenticatedUserID(r.Context(), userID)))
		})
	}
}

func withOptionalAuthContext(r *http.Request, logger *zap.Logger, validator AccessTokenValidator) *http.Request {
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		return r
	}

	claims, err := validator.ValidateAccessToken(token)
	if err != nil {
		logger.Debug("optional access token validation failed", zap.Error(err))
		return r
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		logger.Debug("optional access token has invalid user id", zap.String("user_id", claims.UserID))
		return r
	}

	return r.WithContext(SetAuthenticatedUserID(r.Context(), userID))
}

func pathRequiresAuth(path string, publicPathAllowlist map[string]struct{}) bool {
	normalized := strings.TrimSuffix(path, "/")
	if normalized == "" {
		normalized = "/"
	}

	if !isV1APIPath(normalized) {
		return false
	}

	if _, allowlisted := publicPathAllowlist[normalized]; allowlisted {
		return false
	}

	for publicPath := range publicPathAllowlist {
		if !strings.HasSuffix(publicPath, "*") {
			continue
		}
		prefix := strings.TrimSuffix(publicPath, "*")
		if strings.HasPrefix(normalized, prefix) {
			return false
		}
	}

	return true
}

func isV1APIPath(path string) bool {
	return path == "/api/v1" || strings.HasPrefix(path, "/api/v1/")
}

func bearerToken(headerValue string) (string, bool) {
	parts := strings.Fields(headerValue)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	if parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func writeUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": message})
}
