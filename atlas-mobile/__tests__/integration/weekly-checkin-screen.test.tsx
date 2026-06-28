import React from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import ReactTestRenderer from 'react-test-renderer';
import { WeeklyCheckInScreen } from '../../src/screens/nutrition/WeeklyCheckInScreen';
import {
  getLatestNutritionWeeklyCheckin,
  getNutritionWeightTrend,
  runWeeklyNutritionCheckin,
  upsertNutritionWeightEntry,
} from '../../src/api/services/nutritionService';

jest.mock('../../src/api/services/nutritionService', () => ({
  getLatestNutritionWeeklyCheckin: jest.fn(),
  getNutritionWeightTrend: jest.fn(),
  runWeeklyNutritionCheckin: jest.fn(),
  upsertNutritionWeightEntry: jest.fn(),
}));

jest.mock('../../src/state/AuthContext', () => ({
  useAuth: () => ({
    session: {
      user: {
        id: 'user-1',
        email: 'athlete@atlas.local',
        createdAt: '2026-01-01T00:00:00.000Z',
      },
      tokens: {
        accessToken: 'access-token',
        refreshToken: 'refresh-token',
        tokenType: 'Bearer',
        expiresIn: 900,
      },
    },
    logout: jest.fn(async () => {}),
  }),
}));

function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: Infinity,
      },
    },
  });
}

async function flush(): Promise<void> {
  await ReactTestRenderer.act(async () => {
    await new Promise<void>(resolve => {
      setTimeout(() => resolve(), 0);
    });
  });
}

describe('WeeklyCheckInScreen', () => {
  const mockedGetNutritionWeightTrend =
    getNutritionWeightTrend as jest.MockedFunction<typeof getNutritionWeightTrend>;
  const mockedGetLatestNutritionWeeklyCheckin =
    getLatestNutritionWeeklyCheckin as jest.MockedFunction<typeof getLatestNutritionWeeklyCheckin>;
  const mockedRunWeeklyNutritionCheckin =
    runWeeklyNutritionCheckin as jest.MockedFunction<typeof runWeeklyNutritionCheckin>;
  const mockedUpsertNutritionWeightEntry =
    upsertNutritionWeightEntry as jest.MockedFunction<typeof upsertNutritionWeightEntry>;

  beforeEach(() => {
    jest.clearAllMocks();

    mockedGetNutritionWeightTrend.mockResolvedValue([
      {
        weekStartDate: '2026-02-09',
        entryDate: '2026-02-11',
        weight: 176,
        unit: 'lb',
        weightKg: 79.8323,
      },
      {
        weekStartDate: '2026-02-16',
        entryDate: undefined,
        weight: undefined,
        unit: undefined,
        weightKg: undefined,
      },
      {
        weekStartDate: '2026-02-23',
        entryDate: '2026-02-26',
        weight: 81.2,
        unit: 'kg',
        weightKg: 81.2,
      },
    ]);

    mockedGetLatestNutritionWeeklyCheckin.mockResolvedValue({
      userId: 'user-1',
      week_start: '2026-02-23',
      adherence: 0.86,
      weight_change: -0.2,
      previous_targets: {
        calories_target: 2300,
        protein_g_target: 165,
        carbs_g_target: 240,
        fat_g_target: 70,
      },
      new_targets: {
        calories_target: 2150,
        protein_g_target: 170,
        carbs_g_target: 220,
        fat_g_target: 65,
      },
      calorie_delta: -150,
      goal_pace_kg_per_week: -0.4,
      explanation: 'Weight trend and adherence were on target.',
      createdAt: '2026-02-27T10:00:00.000Z',
      updatedAt: '2026-02-27T10:00:00.000Z',
    });

    mockedRunWeeklyNutritionCheckin.mockResolvedValue({
      userId: 'user-1',
      week_start: '2026-02-23',
      adherence: 0.86,
      weight_change: -0.2,
      previous_targets: {
        calories_target: 2300,
        protein_g_target: 165,
        carbs_g_target: 240,
        fat_g_target: 70,
      },
      new_targets: {
        calories_target: 2150,
        protein_g_target: 170,
        carbs_g_target: 220,
        fat_g_target: 65,
      },
      calorie_delta: -150,
      goal_pace_kg_per_week: -0.4,
      explanation: 'Weight trend and adherence were on target.',
      createdAt: '2026-02-27T10:00:00.000Z',
      updatedAt: '2026-02-27T10:00:00.000Z',
    });

    mockedUpsertNutritionWeightEntry.mockResolvedValue({
      userId: 'user-1',
      date: '2026-02-27',
      weight: 81.5,
      unit: 'kg',
      weightKg: 81.5,
      createdAt: '2026-02-27T10:00:00.000Z',
      updatedAt: '2026-02-27T10:00:00.000Z',
    });
  });

  it('renders trend values for logged and empty weeks', async () => {
    const queryClient = createQueryClient();
    let renderer: ReactTestRenderer.ReactTestRenderer;

    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <WeeklyCheckInScreen />
        </QueryClientProvider>,
      );
    });

    await flush();
    await flush();

    const currentWeekValue = renderer!.root.findByProps({
      testID: 'weight-trend-value-2026-02-23',
    });
    expect(currentWeekValue.props.children).toBe('81.2 kg');

    const beforeCalories = renderer!.root.findByProps({
      testID: 'weekly-checkin-calories-before',
    });
    expect(beforeCalories.props.children).toBe('2300 kcal');

    const afterCalories = renderer!.root.findByProps({
      testID: 'weekly-checkin-calories-target',
    });
    expect(afterCalories.props.children).toBe('2150 kcal');

    const adjustmentSummary = renderer!.root.findByProps({
      testID: 'weekly-checkin-adjustment-summary',
    });
    expect(adjustmentSummary.props.children).toContain('goal pace -0.40 kg/week');

    const emptyWeekValue = renderer!.root.findByProps({
      testID: 'weight-trend-value-2026-02-16',
    });
    expect(emptyWeekValue.props.children).toBe('--');

    const olderWeekValue = renderer!.root.findByProps({
      testID: 'weight-trend-value-2026-02-09',
    });
    expect(olderWeekValue.props.children).toBe('176.0 lb');

    await ReactTestRenderer.act(async () => {
      renderer!.unmount();
    });
    queryClient.clear();
  });
});
