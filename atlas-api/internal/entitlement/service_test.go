package entitlement

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSnapshotListAndChecks(t *testing.T) {
	t.Parallel()

	snapshot := NewSnapshot([]string{
		BarcodeScanEntitlement,
		DeepNutritionEntitlement,
		DeepNutritionEntitlement,
		"  ",
	})

	require.True(t, snapshot.IsPro())
	require.True(t, snapshot.Has(BarcodeScanEntitlement))
	require.True(t, snapshot.Has("DEEP_NUTRITION"))
	require.False(t, snapshot.Has(FormCheckUploadEntitlement))
	require.Equal(
		t,
		[]string{BarcodeScanEntitlement, DeepNutritionEntitlement},
		snapshot.List(),
	)
}

func TestSnapshotCoachTier(t *testing.T) {
	t.Parallel()

	freeSnapshot := NewSnapshot(nil)
	require.Equal(t, CoachTierFree, freeSnapshot.CoachTier())
	require.False(t, freeSnapshot.HasCoachTier(CoachTierPro))

	proSnapshot := NewSnapshot([]string{CoachTierProEntitlement})
	require.Equal(t, CoachTierPro, proSnapshot.CoachTier())
	require.True(t, proSnapshot.HasCoachTier(CoachTierPro))
	require.False(t, proSnapshot.HasCoachTier(CoachTierElite))

	eliteSnapshot := NewSnapshot([]string{CoachTierEliteEntitlement})
	require.Equal(t, CoachTierElite, eliteSnapshot.CoachTier())
	require.True(t, eliteSnapshot.HasCoachTier(CoachTierPro))
	require.True(t, eliteSnapshot.HasCoachTier(CoachTierElite))
}

func TestEffectiveSubscriptionStatus(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC)
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)

	require.Equal(
		t,
		SubscriptionStatusActive,
		EffectiveSubscriptionStatus("", &future, now),
	)
	require.Equal(
		t,
		SubscriptionStatusExpired,
		EffectiveSubscriptionStatus("", &past, now),
	)
	require.Equal(
		t,
		SubscriptionStatusExpired,
		EffectiveSubscriptionStatus(SubscriptionStatusActive, &past, now),
	)
	require.Equal(
		t,
		SubscriptionStatusCanceled,
		EffectiveSubscriptionStatus(SubscriptionStatusCanceled, &future, now),
	)
}
