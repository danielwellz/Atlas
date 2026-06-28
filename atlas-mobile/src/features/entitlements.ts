import type { components } from '../api/generated/openapi';

type User = components['schemas']['User'];
export type EntitlementKey = components['schemas']['EntitlementKey'];
export type CoachTier = components['schemas']['CoachTier'];

export function hasBarcodeScanEntitlement(user: User | null | undefined): boolean {
  return hasEntitlement(user, 'barcode_scan');
}

export function hasDeepNutritionEntitlement(user: User | null | undefined): boolean {
  return hasEntitlement(user, 'deep_nutrition');
}

export function hasBiomechanicsOverlayEntitlement(user: User | null | undefined): boolean {
  return hasEntitlement(user, 'biomechanics_overlays');
}

export function hasFormCheckUploadEntitlement(user: User | null | undefined): boolean {
  return hasEntitlement(user, 'form_check_upload');
}

export function hasEntitlement(
  user: User | null | undefined,
  entitlement: EntitlementKey,
): boolean {
  if (!user) {
    return false;
  }

  const activeEntitlements = user.entitlements ?? [];
  if (activeEntitlements.includes(entitlement)) {
    return true;
  }

  // Backward-compatible fallback for stale payloads.
  return user.isPro && entitlement !== 'coach_tier_elite';
}

export function userCoachTier(user: User | null | undefined): CoachTier {
  if (!user) {
    return 'free';
  }

  if (user.coachTier) {
    return user.coachTier;
  }

  return user.isPro ? 'pro' : 'free';
}

export function canAccessCoachTier(
  user: User | null | undefined,
  requiredTier: CoachTier,
): boolean {
  return coachTierRank(userCoachTier(user)) >= coachTierRank(requiredTier);
}

function coachTierRank(tier: CoachTier): number {
  switch (tier) {
    case 'elite':
      return 2;
    case 'pro':
      return 1;
    default:
      return 0;
  }
}
