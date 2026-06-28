package entitlement

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	BarcodeScanEntitlement        = "barcode_scan"
	DeepNutritionEntitlement      = "deep_nutrition"
	BiomechanicsOverlayEntitlement = "biomechanics_overlays"
	FormCheckUploadEntitlement    = "form_check_upload"
	CoachTierProEntitlement       = "coach_tier_pro"
	CoachTierEliteEntitlement     = "coach_tier_elite"
)

const (
	SubscriptionStatusActive      = "active"
	SubscriptionStatusExpired     = "expired"
	SubscriptionStatusCanceled    = "canceled"
	SubscriptionStatusGracePeriod = "grace_period"
	SubscriptionStatusRefunded    = "refunded"
)

type CoachTier string

const (
	CoachTierFree  CoachTier = "free"
	CoachTierPro   CoachTier = "pro"
	CoachTierElite CoachTier = "elite"
)

type Querier interface {
	ListUserEntitlements(ctx context.Context, userID uuid.UUID) ([]string, error)
}

type Service struct {
	queries Querier
}

func NewService(queries Querier) *Service {
	return &Service{queries: queries}
}

type Snapshot struct {
	values map[string]struct{}
}

func NewSnapshot(entitlements []string) Snapshot {
	values := make(map[string]struct{}, len(entitlements))
	for _, entitlement := range entitlements {
		normalized := normalizeEntitlement(entitlement)
		if normalized == "" {
			continue
		}
		values[normalized] = struct{}{}
	}

	return Snapshot{values: values}
}

func (s *Service) SnapshotForUser(ctx context.Context, userID uuid.UUID) (Snapshot, error) {
	if s == nil || s.queries == nil {
		return NewSnapshot(nil), nil
	}

	entitlements, err := s.queries.ListUserEntitlements(ctx, userID)
	if err != nil {
		return Snapshot{}, err
	}

	return NewSnapshot(entitlements), nil
}

func (s Snapshot) Has(entitlement string) bool {
	if len(s.values) == 0 {
		return false
	}
	_, ok := s.values[normalizeEntitlement(entitlement)]
	return ok
}

func (s Snapshot) IsPro() bool {
	return len(s.values) > 0
}

func (s Snapshot) List() []string {
	if len(s.values) == 0 {
		return []string{}
	}

	keys := make([]string, 0, len(s.values))
	for key := range s.values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (s Snapshot) CoachTier() CoachTier {
	if s.Has(CoachTierEliteEntitlement) {
		return CoachTierElite
	}
	if s.Has(CoachTierProEntitlement) {
		return CoachTierPro
	}
	return CoachTierFree
}

func (s Snapshot) HasCoachTier(required CoachTier) bool {
	return CoachTierRank(s.CoachTier()) >= CoachTierRank(required)
}

func ParseCoachTier(value string) CoachTier {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(CoachTierElite):
		return CoachTierElite
	case string(CoachTierPro):
		return CoachTierPro
	default:
		return CoachTierFree
	}
}

func CoachTierRank(tier CoachTier) int {
	switch tier {
	case CoachTierElite:
		return 2
	case CoachTierPro:
		return 1
	default:
		return 0
	}
}

func EffectiveSubscriptionStatus(status string, expiresAt *time.Time, now time.Time) string {
	normalized := normalizeSubscriptionStatus(status)
	if normalized == "" {
		if expiresAt != nil && expiresAt.After(now) {
			return SubscriptionStatusActive
		}
		return SubscriptionStatusExpired
	}

	if expiresAt != nil && !expiresAt.After(now) {
		if normalized == SubscriptionStatusActive || normalized == SubscriptionStatusGracePeriod {
			return SubscriptionStatusExpired
		}
	}

	return normalized
}

func normalizeEntitlement(entitlement string) string {
	return strings.ToLower(strings.TrimSpace(entitlement))
}

func normalizeSubscriptionStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case SubscriptionStatusActive:
		return SubscriptionStatusActive
	case SubscriptionStatusExpired:
		return SubscriptionStatusExpired
	case SubscriptionStatusCanceled:
		return SubscriptionStatusCanceled
	case SubscriptionStatusGracePeriod:
		return SubscriptionStatusGracePeriod
	case SubscriptionStatusRefunded:
		return SubscriptionStatusRefunded
	default:
		return ""
	}
}
