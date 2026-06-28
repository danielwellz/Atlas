import {
  canAccessCoachTier,
  hasBarcodeScanEntitlement,
  hasBiomechanicsOverlayEntitlement,
  hasDeepNutritionEntitlement,
  hasFormCheckUploadEntitlement,
  userCoachTier,
} from '../../src/features/entitlements';
import type { components } from '../../src/api/generated/openapi';

type User = components['schemas']['User'];

describe('entitlements helpers', () => {
  it('resolves feature entitlement flags from active keys', () => {
    const user: User = {
      id: 'user-1',
      email: 'athlete@atlas.local',
      isPro: true,
      entitlements: [
        'barcode_scan',
        'deep_nutrition',
        'biomechanics_overlays',
        'form_check_upload',
        'coach_tier_pro',
      ],
      coachTier: 'pro',
      createdAt: '2026-01-01T00:00:00.000Z',
    };

    expect(hasBarcodeScanEntitlement(user)).toBe(true);
    expect(hasDeepNutritionEntitlement(user)).toBe(true);
    expect(hasBiomechanicsOverlayEntitlement(user)).toBe(true);
    expect(hasFormCheckUploadEntitlement(user)).toBe(true);
    expect(userCoachTier(user)).toBe('pro');
  });

  it('enforces coach tier rank comparisons', () => {
    const freeUser: User = {
      id: 'free-user',
      email: 'free@atlas.local',
      isPro: false,
      entitlements: [],
      coachTier: 'free',
      createdAt: '2026-01-01T00:00:00.000Z',
    };
    const proUser: User = {
      ...freeUser,
      id: 'pro-user',
      isPro: true,
      entitlements: ['coach_tier_pro'],
      coachTier: 'pro',
    };
    const eliteUser: User = {
      ...freeUser,
      id: 'elite-user',
      isPro: true,
      entitlements: ['coach_tier_pro', 'coach_tier_elite'],
      coachTier: 'elite',
    };

    expect(canAccessCoachTier(freeUser, 'free')).toBe(true);
    expect(canAccessCoachTier(freeUser, 'pro')).toBe(false);
    expect(canAccessCoachTier(proUser, 'pro')).toBe(true);
    expect(canAccessCoachTier(proUser, 'elite')).toBe(false);
    expect(canAccessCoachTier(eliteUser, 'elite')).toBe(true);
  });
});
