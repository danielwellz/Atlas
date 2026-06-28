package httpapi

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	db "github.com/atlas/atlas-api/internal/db/sqlc"
	"github.com/atlas/atlas-api/internal/entitlement"
	"github.com/atlas/atlas-api/internal/httpapi/generated"
	"github.com/atlas/atlas-api/internal/httpapi/middleware"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"
)

const defaultMonthlySubscriptionDuration = 30 * 24 * time.Hour
const defaultYearlySubscriptionDuration = 365 * 24 * time.Hour

func (s *Server) PostBillingVerify(
	ctx context.Context,
	request generated.PostBillingVerifyRequestObject,
) (generated.PostBillingVerifyResponseObject, error) {
	if request.Body == nil {
		return generated.PostBillingVerify400JSONResponse{Message: "request body is required"}, nil
	}

	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PostBillingVerify401JSONResponse{Message: "unauthorized"}, nil
	}

	productID := strings.TrimSpace(request.Body.ProductId)
	if productID == "" {
		return generated.PostBillingVerify400JSONResponse{Message: "productId is required"}, nil
	}

	receiptToken := strings.TrimSpace(request.Body.ReceiptToken)
	if receiptToken == "" {
		return generated.PostBillingVerify400JSONResponse{Message: "receiptToken is required"}, nil
	}

	platform := strings.ToLower(strings.TrimSpace(string(request.Body.Platform)))
	switch platform {
	case "ios", "android":
	default:
		return generated.PostBillingVerify400JSONResponse{Message: "platform must be ios or android"}, nil
	}

	now := s.currentTime().UTC()
	resolvedExpiry := resolveSubscriptionExpiry(now, productID, request.Body.ExpiresAt)
	effectiveStatus := entitlement.EffectiveSubscriptionStatus(
		entitlement.SubscriptionStatusActive,
		nullTimeToPointer(resolvedExpiry),
		now,
	)

	transactionID := strings.TrimSpace(nullableStringValue(request.Body.TransactionId))
	if transactionID == "" {
		transactionID = deriveTransactionID(platform, productID, receiptToken, request.Body.OriginalTransactionId)
	}

	rawReceiptPayload := map[string]interface{}{
		"platform":     platform,
		"productId":    productID,
		"receiptToken": receiptToken,
		"transactionId": transactionID,
		"verifiedAt":   now.UTC().Format(time.RFC3339),
	}
	if request.Body.OriginalTransactionId != nil {
		rawReceiptPayload["originalTransactionId"] = strings.TrimSpace(*request.Body.OriginalTransactionId)
	}
	if request.Body.Restore != nil {
		rawReceiptPayload["restore"] = *request.Body.Restore
	}
	if request.Body.ExpiresAt != nil {
		rawReceiptPayload["expiresAt"] = request.Body.ExpiresAt.UTC().Format(time.RFC3339)
	}

	rawReceiptJSON, err := json.Marshal(rawReceiptPayload)
	if err != nil {
		s.logger.Error("failed marshaling billing receipt payload", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PostBillingVerify400JSONResponse{Message: "invalid billing payload"}, nil
	}

	subscriptionRow, err := s.queries.UpsertSubscription(ctx, db.UpsertSubscriptionParams{
		UserID:                userID,
		Platform:              platform,
		ProductID:             productID,
		Status:                effectiveStatus,
		ExpiresAt:             resolvedExpiry,
		RawReceipt:            rawReceiptJSON,
		TransactionID:         transactionID,
		OriginalTransactionID: nullableStringParam(request.Body.OriginalTransactionId),
	})
	if err != nil {
		s.logger.Error(
			"failed upserting subscription during billing verify",
			zap.Error(err),
			zap.String("user_id", userID.String()),
			zap.String("platform", platform),
			zap.String("product_id", productID),
			zap.String("transaction_id", transactionID),
		)
		return generated.PostBillingVerify500JSONResponse{Message: "could not verify subscription"}, nil
	}

	snapshot := entitlement.NewSnapshot(nil)
	if s.entitlement != nil {
		snapshot, err = s.entitlement.SnapshotForUser(ctx, userID)
		if err != nil {
			s.logger.Error(
				"failed loading entitlements after billing verify",
				zap.Error(err),
				zap.String("user_id", userID.String()),
			)
			return generated.PostBillingVerify500JSONResponse{Message: "could not verify subscription"}, nil
		}
	}

	restored := request.Body.Restore != nil && *request.Body.Restore

	return generated.PostBillingVerify200JSONResponse{
		Subscription: toAPISubscription(subscriptionRow),
		Entitlements: toAPIEntitlementKeys(snapshot.List()),
		CoachTier:    generated.CoachTier(snapshot.CoachTier()),
		IsPro:        snapshot.IsPro(),
		Restored:     restored,
	}, nil
}

func toAPISubscription(row db.Subscription) generated.Subscription {
	var expiresAt *time.Time
	if row.ExpiresAt.Valid {
		value := row.ExpiresAt.Time
		expiresAt = &value
	}

	var originalTransactionID *string
	if row.OriginalTransactionID.Valid {
		value := strings.TrimSpace(row.OriginalTransactionID.String)
		if value != "" {
			originalTransactionID = &value
		}
	}

	rawReceipt := map[string]interface{}{}
	if len(row.RawReceipt) > 0 {
		_ = json.Unmarshal(row.RawReceipt, &rawReceipt)
	}

	return generated.Subscription{
		Id:                    openapi_types.UUID(row.ID),
		UserId:                openapi_types.UUID(row.UserID),
		Platform:              generated.BillingPlatform(row.Platform),
		ProductId:             row.ProductID,
		Status:                generated.SubscriptionStatus(row.Status),
		ExpiresAt:             expiresAt,
		TransactionId:         row.TransactionID,
		OriginalTransactionId: originalTransactionID,
		RawReceipt:            rawReceipt,
		CreatedAt:             row.CreatedAt,
		UpdatedAt:             row.UpdatedAt,
	}
}

func resolveSubscriptionExpiry(now time.Time, productID string, requestedExpiry *time.Time) sql.NullTime {
	if requestedExpiry != nil {
		return sql.NullTime{
			Time:  requestedExpiry.UTC(),
			Valid: true,
		}
	}

	duration := defaultMonthlySubscriptionDuration
	normalizedProduct := strings.ToLower(strings.TrimSpace(productID))
	if strings.Contains(normalizedProduct, "year") || strings.Contains(normalizedProduct, "annual") {
		duration = defaultYearlySubscriptionDuration
	}

	return sql.NullTime{
		Time:  now.Add(duration),
		Valid: true,
	}
}

func nullTimeToPointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}

	timestamp := value.Time
	return &timestamp
}

func nullableStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func deriveTransactionID(platform, productID, receiptToken string, originalTransactionID *string) string {
	base := platform + "|" + productID + "|" + strings.TrimSpace(receiptToken) + "|" + nullableStringValue(originalTransactionID)
	digest := sha256.Sum256([]byte(base))
	return hex.EncodeToString(digest[:16])
}
