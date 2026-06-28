package middleware

import (
	"context"

	"github.com/google/uuid"
)

type authContextKey string

const authenticatedUserIDKey authContextKey = "atlas-authenticated-user-id"

func SetAuthenticatedUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, authenticatedUserIDKey, userID)
}

func AuthenticatedUserID(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(authenticatedUserIDKey).(uuid.UUID)
	if !ok {
		return uuid.UUID{}, false
	}
	return userID, true
}
