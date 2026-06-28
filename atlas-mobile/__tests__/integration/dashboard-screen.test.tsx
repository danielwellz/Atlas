import React from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import ReactTestRenderer from 'react-test-renderer';
import { DashboardScreen } from '../../src/screens/dashboard/DashboardScreen';
import { getDashboardSummary } from '../../src/api/services/dashboardService';
import {
  completeMomentumSprintChecklistEntry,
  getMomentumSprintStatus,
  listHabits,
  toggleHabitDailyLog,
} from '../../src/api/services/habitService';
import {
  getNutritionTargets,
  upsertDailyNutritionCheckin,
} from '../../src/api/services/nutritionService';
import { upsertDashboardReadinessCheckin } from '../../src/api/services/readinessService';

jest.mock('@react-navigation/native', () => ({
  useNavigation: () => ({
    navigate: jest.fn(),
  }),
}));

jest.mock('../../src/api/services/dashboardService', () => ({
  getDashboardSummary: jest.fn(),
}));

jest.mock('../../src/api/services/habitService', () => ({
  completeMomentumSprintChecklistEntry: jest.fn(),
  getMomentumSprintStatus: jest.fn(),
  listHabits: jest.fn(),
  toggleHabitDailyLog: jest.fn(),
}));

jest.mock('../../src/api/services/nutritionService', () => ({
  getNutritionTargets: jest.fn(),
  upsertDailyNutritionCheckin: jest.fn(),
}));

jest.mock('../../src/api/services/readinessService', () => ({
  upsertDashboardReadinessCheckin: jest.fn(),
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

let mockedOnboardingState: {
  firstWeekPlan: { day: string; sessionName: string }[] | null;
  planExplanation: string | null;
};

jest.mock('../../src/state/OnboardingContext', () => ({
  useOnboarding: () => ({
    firstWeekPlan:
      mockedOnboardingState.firstWeekPlan === null
        ? null
        : { days: mockedOnboardingState.firstWeekPlan },
    planExplanation: mockedOnboardingState.planExplanation,
  }),
}));

const TODAY_KEY = new Date().toISOString().slice(0, 10);

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

async function cleanup(
  renderer: ReactTestRenderer.ReactTestRenderer,
  queryClient: QueryClient,
): Promise<void> {
  await ReactTestRenderer.act(async () => {
    renderer.unmount();
  });
  queryClient.clear();
}

describe('DashboardScreen integration', () => {
  const mockedGetDashboardSummary = getDashboardSummary as jest.MockedFunction<
    typeof getDashboardSummary
  >;
  const mockedListHabits = listHabits as jest.MockedFunction<typeof listHabits>;
  const mockedToggleHabitDailyLog = toggleHabitDailyLog as jest.MockedFunction<
    typeof toggleHabitDailyLog
  >;
  const mockedGetMomentumSprintStatus =
    getMomentumSprintStatus as jest.MockedFunction<
      typeof getMomentumSprintStatus
    >;
  const mockedCompleteMomentumSprintChecklistEntry =
    completeMomentumSprintChecklistEntry as jest.MockedFunction<
      typeof completeMomentumSprintChecklistEntry
    >;
  const mockedGetNutritionTargets = getNutritionTargets as jest.MockedFunction<
    typeof getNutritionTargets
  >;
  const mockedUpsertDailyNutritionCheckin =
    upsertDailyNutritionCheckin as jest.MockedFunction<
      typeof upsertDailyNutritionCheckin
    >;
  const mockedUpsertDashboardReadinessCheckin =
    upsertDashboardReadinessCheckin as jest.MockedFunction<
      typeof upsertDashboardReadinessCheckin
    >;

  beforeEach(() => {
    jest.clearAllMocks();
    mockedOnboardingState = {
      firstWeekPlan: null,
      planExplanation: null,
    };

    mockedGetDashboardSummary.mockResolvedValue({
      workoutsCompletedLast7Days: 3,
      totalSetsLast7Days: 12,
      volumeByMovementPatternLast7Days: {
        squat: 420,
      },
      prs: {
        squat: {
          best5RmEstimateKg: 120,
          bestSetReps: 5,
          bestSetWeightKg: 125,
        },
        bench: {
          best5RmEstimateKg: 85,
          bestSetReps: 5,
          bestSetWeightKg: 90,
        },
        deadlift: {
          best5RmEstimateKg: 150,
          bestSetReps: 5,
          bestSetWeightKg: 160,
        },
      },
      estimatedOneRmByLift: {
        squat: {
          estimatedOneRmKg: 132,
          bestSetReps: 3,
          bestSetWeightKg: 120,
          achievedAt: '2026-02-26T10:00:00.000Z',
        },
        bench: {
          estimatedOneRmKg: 93.33,
          bestSetReps: 5,
          bestSetWeightKg: 80,
          achievedAt: '2026-02-25T10:00:00.000Z',
        },
        deadlift: {
          estimatedOneRmKg: 181.33,
          bestSetReps: 2,
          bestSetWeightKg: 170,
          achievedAt: '2026-02-07T10:00:00.000Z',
        },
      },
      prEvents: [
        {
          lift: 'squat',
          completedAt: '2026-02-26T10:00:00.000Z',
          reps: 3,
          weightKg: 120,
          estimatedOneRmKg: 132,
          previousEstimatedOneRmKg: 116.66,
          improvementKg: 15.34,
        },
      ],
      weeklyMuscleGroupVolume: [
        {
          weekStartDate: '2026-02-23',
          volumeByMuscleGroup: {
            quads: 430,
            glutes: 215,
          },
        },
        {
          weekStartDate: '2026-02-16',
          volumeByMuscleGroup: {},
        },
      ],
      weeklyVolumeTrend: [
        {
          weekStartDate: '2026-02-23',
          totalVolumeKg: 1260,
        },
        {
          weekStartDate: '2026-02-16',
          totalVolumeKg: 840,
        },
      ],
      adherenceStreaks: {
        training: {
          currentDays: 2,
          longestDays: 4,
        },
        protein: {
          currentDays: 1,
          longestDays: 3,
        },
      },
      weightTrendPoints: [
        {
          date: '2026-02-06',
          weightKg: 84.4,
        },
        {
          date: '2026-02-13',
          weightKg: 84.0,
        },
        {
          date: '2026-02-20',
          weightKg: 83.7,
        },
      ],
      readinessSelfReportHistory: [
        {
          date: '2026-02-25',
          readinessScore: 2.67,
          energyLevel: 3,
          sleepQuality: 2,
          stressLevel: 1,
        },
      ],
      proteinAdherenceLast7DaysPercent: 57.4,
      nutritionTotalsToday: {
        calories_kcal: 2050,
        protein_g: 165,
        carbs_g: 210,
        fat_g: 72,
      },
    });

    mockedGetNutritionTargets.mockResolvedValue({
      date: TODAY_KEY,
      targetsConfigured: true,
      targets: {
        userId: 'user-1',
        calories_target: 2200,
        protein_g_target: 160,
        createdAt: '2026-01-01T00:00:00.000Z',
        updatedAt: '2026-01-01T00:00:00.000Z',
      },
      checkin: undefined,
    });

    mockedUpsertDailyNutritionCheckin.mockResolvedValue({
      id: 'checkin-1',
      userId: 'user-1',
      date: TODAY_KEY,
      calories_estimate: 2100,
      protein_g_estimate: 170,
      hit_calories: true,
      hit_protein: true,
      notes: '',
      createdAt: '2026-02-26T10:00:00.000Z',
    });
    mockedUpsertDashboardReadinessCheckin.mockResolvedValue({
      userId: 'user-1',
      date: TODAY_KEY,
      energyLevel: 2,
      sleepQuality: 2,
      stressLevel: 2,
      readinessScore: 2,
      createdAt: '2026-02-26T10:00:00.000Z',
      updatedAt: '2026-02-26T10:00:00.000Z',
    });

    mockedGetMomentumSprintStatus.mockResolvedValue({
      enrolled: false,
      enrollment: undefined,
      progress: undefined,
      todayChecklist: [],
      milestones: [],
    });

    mockedCompleteMomentumSprintChecklistEntry.mockResolvedValue({
      enrolled: false,
      enrollment: undefined,
      progress: undefined,
      todayChecklist: [],
      milestones: [],
    });
  });

  it('toggles a habit and updates the UI from refetched server data', async () => {
    const queryClient = createQueryClient();

    let habitCompleted = false;
    mockedListHabits.mockImplementation(async () => [
      {
        id: 'habit-water',
        userId: 'user-1',
        name: 'Hydration',
        type: 'daily',
        targetJson: { count: 1 },
        active: true,
        completed: habitCompleted,
        createdAt: '2026-01-01T00:00:00.000Z',
      },
    ]);

    mockedToggleHabitDailyLog.mockImplementation(async ({ habitId }) => {
      habitCompleted = !habitCompleted;

      return {
        id: 'habit-log-1',
        habitId,
        date: TODAY_KEY,
        completed: habitCompleted,
        completedAt: habitCompleted ? '2026-02-26T10:00:00.000Z' : null,
        createdAt: '2026-02-26T10:00:00.000Z',
      };
    });

    let renderer: ReactTestRenderer.ReactTestRenderer;

    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <DashboardScreen />
        </QueryClientProvider>,
      );
    });

    await flush();
    await flush();

    expect(
      renderer!.root.findByProps({ testID: 'dashboard-estimated-1rm' }),
    ).toBeDefined();
    expect(
      renderer!.root.findByProps({ testID: 'dashboard-pr-highlights' }),
    ).toBeDefined();
    expect(
      renderer!.root.findByProps({ testID: 'dashboard-volume-trends' }),
    ).toBeDefined();
    expect(
      renderer!.root.findByProps({
        testID: 'dashboard-muscle-volume-distribution',
      }),
    ).toBeDefined();
    expect(
      renderer!.root.findByProps({ testID: 'dashboard-weight-trend' }),
    ).toBeDefined();
    expect(
      renderer!.root.findByProps({ testID: 'dashboard-readiness-history' }),
    ).toBeDefined();

    const initialButton = renderer!.root.findByProps({
      testID: 'habit-toggle-habit-water',
    });
    expect(initialButton.props.label).toBe('Mark');

    await ReactTestRenderer.act(async () => {
      initialButton.props.onPress();
    });

    expect(mockedToggleHabitDailyLog).toHaveBeenCalledWith({
      accessToken: 'access-token',
      habitId: 'habit-water',
    });

    await flush();
    await flush();

    const updatedButton = renderer!.root.findByProps({
      testID: 'habit-toggle-habit-water',
    });
    expect(updatedButton.props.label).toBe('Done');
    expect(mockedListHabits.mock.calls.length).toBeGreaterThanOrEqual(2);

    await cleanup(renderer!, queryClient);
  });

  it('renders onboarding first-week plan explanation when available', async () => {
    const queryClient = createQueryClient();
    mockedOnboardingState = {
      firstWeekPlan: [{ day: 'Monday', sessionName: 'Strength Builder 1' }],
      planExplanation:
        'Plan built from schedule, modality preferences, and limitations.',
    };

    mockedListHabits.mockResolvedValue([]);
    mockedToggleHabitDailyLog.mockResolvedValue({
      id: 'habit-log-1',
      habitId: 'habit-water',
      date: TODAY_KEY,
      completed: true,
      completedAt: '2026-02-26T10:00:00.000Z',
      createdAt: '2026-02-26T10:00:00.000Z',
    });

    let renderer: ReactTestRenderer.ReactTestRenderer;
    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <DashboardScreen />
        </QueryClientProvider>,
      );
    });

    await flush();
    await flush();

    expect(
      renderer!.root.findByProps({ testID: 'first-week-plan-summary' }),
    ).toBeDefined();
    expect(JSON.stringify(renderer!.toJSON())).toContain(
      'Plan built from schedule, modality preferences, and limitations.',
    );
    expect(JSON.stringify(renderer!.toJSON())).toContain('Strength Builder 1');

    await cleanup(renderer!, queryClient);
  });

  it('renders momentum sprint checklist progress and updates entry completion', async () => {
    const queryClient = createQueryClient();

    let sprintChecklistCompleted = false;
    mockedListHabits.mockResolvedValue([]);
    mockedGetMomentumSprintStatus.mockImplementation(async () => ({
      enrolled: true,
      enrollment: {
        id: 'sprint-1',
        userId: 'user-1',
        goal: 'build_strength',
        startDate: TODAY_KEY,
        endDate: '2026-03-12',
        completedAt: null,
        createdAt: '2026-02-27T10:00:00.000Z',
      },
      progress: {
        totalDays: 14,
        completedDays: sprintChecklistCompleted ? 1 : 0,
        currentDay: 1,
        daysRemaining: sprintChecklistCompleted ? 13 : 14,
        completionPercent: sprintChecklistCompleted ? 7.14 : 0,
        currentStreak: sprintChecklistCompleted ? 1 : 0,
        longestStreak: sprintChecklistCompleted ? 1 : 0,
        completedToday: sprintChecklistCompleted,
        nextMilestoneDay: 3,
        nextMilestoneLabel: '3-Day Ignition',
      },
      todayChecklist: [
        {
          id: 'entry-1',
          date: TODAY_KEY,
          habitKey: 'training_session',
          habitLabel: 'Complete your strength session',
          displayOrder: 0,
          completed: sprintChecklistCompleted,
          completedAt: sprintChecklistCompleted
            ? '2026-02-27T10:00:00.000Z'
            : null,
          createdAt: '2026-02-27T10:00:00.000Z',
        },
      ],
      milestones: [],
    }));
    mockedCompleteMomentumSprintChecklistEntry.mockImplementation(
      async input => {
        sprintChecklistCompleted = input.completed;

        return {
          enrolled: true,
          enrollment: {
            id: 'sprint-1',
            userId: 'user-1',
            goal: 'build_strength',
            startDate: TODAY_KEY,
            endDate: '2026-03-12',
            completedAt: null,
            createdAt: '2026-02-27T10:00:00.000Z',
          },
          progress: {
            totalDays: 14,
            completedDays: sprintChecklistCompleted ? 1 : 0,
            currentDay: 1,
            daysRemaining: sprintChecklistCompleted ? 13 : 14,
            completionPercent: sprintChecklistCompleted ? 7.14 : 0,
            currentStreak: sprintChecklistCompleted ? 1 : 0,
            longestStreak: sprintChecklistCompleted ? 1 : 0,
            completedToday: sprintChecklistCompleted,
            nextMilestoneDay: 3,
            nextMilestoneLabel: '3-Day Ignition',
          },
          todayChecklist: [
            {
              id: 'entry-1',
              date: TODAY_KEY,
              habitKey: 'training_session',
              habitLabel: 'Complete your strength session',
              displayOrder: 0,
              completed: sprintChecklistCompleted,
              completedAt: sprintChecklistCompleted
                ? '2026-02-27T10:00:00.000Z'
                : null,
              createdAt: '2026-02-27T10:00:00.000Z',
            },
          ],
          milestones: [],
        };
      },
    );

    let renderer: ReactTestRenderer.ReactTestRenderer;
    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <DashboardScreen />
        </QueryClientProvider>,
      );
    });

    await flush();
    await flush();

    expect(
      renderer!.root.findByProps({ testID: 'momentum-sprint-card' }),
    ).toBeDefined();
    const sprintButton = renderer!.root.findByProps({
      testID: 'momentum-sprint-toggle-training_session',
    });
    expect(sprintButton.props.label).toBe('Check');

    await ReactTestRenderer.act(async () => {
      sprintButton.props.onPress();
    });

    expect(mockedCompleteMomentumSprintChecklistEntry).toHaveBeenCalledWith({
      accessToken: 'access-token',
      habitKey: 'training_session',
      completed: true,
      dateKey: TODAY_KEY,
    });

    await flush();
    await flush();

    const updatedSprintButton = renderer!.root.findByProps({
      testID: 'momentum-sprint-toggle-training_session',
    });
    expect(updatedSprintButton.props.label).toBe('Checked');

    await cleanup(renderer!, queryClient);
  });

  it('submits readiness check-in selections', async () => {
    const queryClient = createQueryClient();
    mockedListHabits.mockResolvedValue([]);

    let renderer: ReactTestRenderer.ReactTestRenderer;
    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <DashboardScreen />
        </QueryClientProvider>,
      );
    });

    await flush();
    await flush();

    await ReactTestRenderer.act(async () => {
      renderer!.root
        .findByProps({ testID: 'readiness-energy-2' })
        .props.onPress();
      renderer!.root
        .findByProps({ testID: 'readiness-sleep-2' })
        .props.onPress();
      renderer!.root
        .findByProps({ testID: 'readiness-stress-2' })
        .props.onPress();
    });

    await ReactTestRenderer.act(async () => {
      renderer!.root
        .findByProps({ testID: 'readiness-submit-button' })
        .props.onPress();
    });

    expect(mockedUpsertDashboardReadinessCheckin).toHaveBeenCalledWith({
      accessToken: 'access-token',
      dateKey: TODAY_KEY,
      energyLevel: 2,
      sleepQuality: 2,
      stressLevel: 2,
    });

    await cleanup(renderer!, queryClient);
  });
});
