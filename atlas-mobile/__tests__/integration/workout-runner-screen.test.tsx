import React from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import ReactTestRenderer from 'react-test-renderer';
import type { components } from '../../src/api/generated/openapi';
import {
  addWorkoutSetLog,
  completeWorkout,
  getExerciseSubstitutes,
  getCurrentSessionPlan,
  getWorkoutDetail,
  getWorkoutHistory,
  startWorkout,
} from '../../src/api/services/workoutService';
import { getExerciseBiomechanics } from '../../src/api/services/biomechanicsService';
import { WorkoutRunnerScreen } from '../../src/screens/workout/WorkoutRunnerScreen';
import { loadExerciseBiomechanics } from '../../src/native/anatomyEngineBridge';
import { openUnity } from '../../src/native/unityBridge';

jest.mock('@react-navigation/native', () => ({
  useNavigation: () => ({
    navigate: jest.fn(),
  }),
}));

jest.mock('../../src/api/services/workoutService', () => {
  const actual = jest.requireActual('../../src/api/services/workoutService');

  return {
    ...actual,
    getCurrentSessionPlan: jest.fn(),
    startWorkout: jest.fn(),
    addWorkoutSetLog: jest.fn(),
    getExerciseSubstitutes: jest.fn(),
    completeWorkout: jest.fn(),
    getWorkoutDetail: jest.fn(),
    getWorkoutHistory: jest.fn(),
  };
});

jest.mock('../../src/api/services/biomechanicsService', () => ({
  getExerciseBiomechanics: jest.fn(),
}));

jest.mock('../../src/native/unityBridge', () => ({
  openUnity: jest.fn(),
  closeUnity: jest.fn(),
  receiveMessageFromUnity: jest.fn(() => () => undefined),
}));

jest.mock('../../src/native/anatomyEngineBridge', () => ({
  loadExerciseBiomechanics: jest.fn(),
}));

jest.mock('../../src/state/AuthContext', () => ({
  useAuth: () => ({
    session: {
      user: {
        id: 'user-1',
        email: 'athlete@atlas.local',
        isPro: true,
        entitlements: ['biomechanics_overlays', 'coach_tier_pro'],
        coachTier: 'pro',
        createdAt: '2026-01-01T00:00:00.000Z',
      },
      tokens: {
        accessToken: 'access-token',
        refreshToken: 'refresh-token',
        tokenType: 'Bearer',
        expiresIn: 900,
      },
    },
  }),
}));

jest.mock('../../src/state/MockModeContext', () => ({
  useMockMode: () => ({
    isMockMode: false,
    canUseMockMode: false,
  }),
}));

type Workout = components['schemas']['Workout'];
type WorkoutSessionPlan = Awaited<ReturnType<typeof getCurrentSessionPlan>>;
type WorkoutSet = components['schemas']['WorkoutSet'];
type ExerciseSubstitute = components['schemas']['ExerciseSubstitute'];

const SESSION_PLAN: NonNullable<WorkoutSessionPlan> = {
  enrollment: {
    id: '7a111111-1111-4111-8111-111111111111',
    userId: '7a222222-2222-4222-8222-222222222222',
    programId: '7a333333-3333-4333-8333-333333333333',
    startDate: '2026-02-26',
    currentWeek: 1,
    createdAt: '2026-02-26T00:00:00.000Z',
  },
  program: {
    id: '7a333333-3333-4333-8333-333333333333',
    slug: 'hypertrophy-foundations',
    name: 'Hypertrophy Foundations',
    description: 'Mock description',
    goalTags: ['hypertrophy'],
    level: 'beginner',
    weeksLength: 8,
    weeklyFrequency: 3,
    blocks: [
      {
        id: '7a555555-1111-4111-8111-111111111111',
        weekIndex: 1,
        sessionCount: 3,
        sessionDays: [2, 4, 6],
      },
    ],
    createdAt: '2026-01-01T00:00:00.000Z',
  },
  week: {
    id: '7a444444-4444-4444-8444-444444444444',
    weekIndex: 1,
    sessions: [
      {
        id: 'session-1',
        dayOfWeek: 4,
        name: 'Lower Strength',
        exercises: [
          {
            id: 'session-exercise-1',
            orderIndex: 1,
            exerciseId: 'exercise-1',
            exerciseSlug: 'back-squat',
            exerciseName: 'Back Squat',
            recommendedLoadKg: 102.5,
            progressionWhy: 'Last week hit target reps at manageable effort; increasing load by 2.5%.',
            prescription: {
              sets: 2,
              reps_range: '5-8',
              rest_seconds: 120,
            },
            substitutionCandidates: [],
          },
        ],
      },
    ],
  },
  session: {
    id: 'session-1',
    dayOfWeek: 4,
    name: 'Lower Strength',
    exercises: [
      {
        id: 'session-exercise-1',
        orderIndex: 1,
        exerciseId: 'exercise-1',
        exerciseSlug: 'back-squat',
        exerciseName: 'Back Squat',
        recommendedLoadKg: 102.5,
        progressionWhy: 'Last week hit target reps at manageable effort; increasing load by 2.5%.',
        prescription: {
          sets: 2,
          reps_range: '5-8',
          rest_seconds: 120,
        },
        substitutionCandidates: [],
      },
    ],
  },
  dayLabel: 'Thursday',
};

const STARTED_WORKOUT: Workout = {
  id: 'workout-1',
  userId: 'user-1',
  programSessionId: 'session-1',
  startedAt: '2026-02-26T12:00:00.000Z',
  completedAt: null,
  notes: '',
  createdAt: '2026-02-26T12:00:00.000Z',
  exercises: [
    {
      id: 'workout-exercise-1',
      workoutId: 'workout-1',
      exerciseId: 'exercise-1',
      orderIndex: 1,
      plannedJson: {
        sets: 2,
        reps_range: '5-8',
        rest_seconds: 120,
        previous_performance: {
          workout_id: 'workout-prev',
          completed_at: '2026-02-24T12:00:00.000Z',
          sets: [
            {
              set_index: 1,
              reps: 7,
              weight_kg: 100,
              rpe: 8,
            },
            {
              set_index: 2,
              reps: 6,
              weight_kg: 102.5,
              rpe: 8.5,
            },
          ],
        },
      },
      actualJson: {},
      createdAt: '2026-02-26T12:00:00.000Z',
      exerciseSlug: 'back-squat',
      exerciseName: 'Back Squat',
      sets: [],
    },
  ],
};

const SUBSTITUTE_OPTIONS: ExerciseSubstitute[] = [
  {
    exercise: {
      id: 'exercise-2',
      slug: 'goblet-squat',
      name: 'Goblet Squat',
      primaryMuscleGroup: 'quads',
      primaryMuscles: ['quads', 'glutes'],
      secondaryMuscles: ['core'],
      movementPattern: 'squat',
      contraindications: [],
      equipment: ['dumbbell', 'kettlebell'],
      difficulty: 'beginner',
      description: 'Front-loaded squat variation.',
      createdAt: '2026-02-20T00:00:00.000Z',
      media: [],
    },
    why: {
      matchedPattern: ['squat'],
      matchedMuscles: ['quads', 'glutes'],
      equipmentFit: 'partial',
    },
  },
];

const BIOMECH_PREVIEW_RESPONSE = {
  exerciseId: 'exercise-1',
  exerciseSlug: 'back-squat',
  exerciseName: 'Back Squat',
  animationAssetKey: 'biomechanics/back-squat/clip_v1.fbx',
  animationAssetUri: 's3://atlas-assets/biomechanics/back-squat/clip_v1.fbx',
  rigVersion: 'atlas-humanoid-v1',
  muscleHighlights: [
    { muscleGroup: 'quads', activationLevel: 1, role: 'primary', colorHex: '#FF6B35' },
  ],
  jointAngles: [{ joint: 'knee', minDegrees: 70, maxDegrees: 175, targetDegrees: 95, unit: 'deg' }],
  metadata: { source: 'test' },
};

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

describe('WorkoutRunnerScreen integration', () => {
  const mockedGetCurrentSessionPlan = getCurrentSessionPlan as jest.MockedFunction<
    typeof getCurrentSessionPlan
  >;
  const mockedStartWorkout = startWorkout as jest.MockedFunction<typeof startWorkout>;
  const mockedAddWorkoutSetLog = addWorkoutSetLog as jest.MockedFunction<typeof addWorkoutSetLog>;
  const mockedGetExerciseSubstitutes = getExerciseSubstitutes as jest.MockedFunction<
    typeof getExerciseSubstitutes
  >;
  const mockedCompleteWorkout = completeWorkout as jest.MockedFunction<typeof completeWorkout>;
  const mockedGetWorkoutDetail = getWorkoutDetail as jest.MockedFunction<typeof getWorkoutDetail>;
  const mockedGetWorkoutHistory = getWorkoutHistory as jest.MockedFunction<typeof getWorkoutHistory>;
  const mockedGetExerciseBiomechanics = getExerciseBiomechanics as jest.MockedFunction<
    typeof getExerciseBiomechanics
  >;
  const mockedOpenUnity = openUnity as jest.MockedFunction<typeof openUnity>;
  const mockedLoadExerciseBiomechanics = loadExerciseBiomechanics as jest.MockedFunction<
    typeof loadExerciseBiomechanics
  >;

  beforeEach(() => {
    jest.clearAllMocks();

    mockedGetCurrentSessionPlan.mockResolvedValue(SESSION_PLAN);
    mockedStartWorkout.mockResolvedValue(STARTED_WORKOUT);
    mockedGetExerciseSubstitutes.mockResolvedValue(SUBSTITUTE_OPTIONS);
    mockedCompleteWorkout.mockResolvedValue({
      ...STARTED_WORKOUT,
      completedAt: '2026-02-26T13:00:00.000Z',
    });
    mockedGetWorkoutDetail.mockResolvedValue(STARTED_WORKOUT);
    mockedGetWorkoutHistory.mockResolvedValue({
      workouts: [],
      next_cursor: null,
    });
    mockedGetExerciseBiomechanics.mockResolvedValue(BIOMECH_PREVIEW_RESPONSE);
    mockedOpenUnity.mockResolvedValue(undefined);
    mockedLoadExerciseBiomechanics.mockResolvedValue(undefined);
  });

  it('shows per-set loading state and prevents duplicate set submission', async () => {
    const queryClient = createQueryClient();
    let resolveAddSet: ((value: WorkoutSet) => void) | null = null;

    mockedAddWorkoutSetLog.mockImplementation(
      () =>
        new Promise<WorkoutSet>(resolve => {
          resolveAddSet = resolve;
        }),
    );

    let renderer: ReactTestRenderer.ReactTestRenderer;

    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <WorkoutRunnerScreen />
        </QueryClientProvider>,
      );
    });

    await flush();
    await flush();

    await ReactTestRenderer.act(async () => {
      renderer!.root
        .findByProps({ testID: 'set-reps-workout-exercise-1-1' })
        .props.onChangeText('8');
      renderer!.root
        .findByProps({ testID: 'set-weight-workout-exercise-1-1' })
        .props.onChangeText('80');
    });

    await ReactTestRenderer.act(async () => {
      const button = renderer!.root.findByProps({ testID: 'log-set-workout-exercise-1-1' });
      button.props.onPress();
      button.props.onPress();
    });

    expect(mockedAddWorkoutSetLog).toHaveBeenCalledTimes(1);
    const firstCall = mockedAddWorkoutSetLog.mock.calls[0];
    expect(firstCall[0].idempotencyKey).toBe('set-workout-1-workout-exercise-1-1');
    expect(firstCall[1]).toBe(false);

    const pendingButton = renderer!.root.findByProps({ testID: 'log-set-workout-exercise-1-1' });
    expect(pendingButton.props.loading).toBe(true);
    expect(pendingButton.props.disabled).toBe(true);

    await ReactTestRenderer.act(async () => {
      resolveAddSet?.({
        id: 'set-1',
        workoutExerciseId: 'workout-exercise-1',
        setIndex: 1,
        reps: 8,
        weightKg: 80,
        rpe: null,
        completedAt: '2026-02-26T12:10:00.000Z',
        createdAt: '2026-02-26T12:10:00.000Z',
      });
    });

    await flush();

    const completedButton = renderer!.root.findByProps({ testID: 'log-set-workout-exercise-1-1' });
    expect(completedButton.props.label).toBe('Done');
    expect(completedButton.props.disabled).toBe(true);

    await cleanup(renderer!, queryClient);
  });

  it('autofills from previous performance and applies nudge suggestion', async () => {
    const queryClient = createQueryClient();

    let renderer: ReactTestRenderer.ReactTestRenderer;
    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <WorkoutRunnerScreen />
        </QueryClientProvider>,
      );
    });

    await flush();
    await flush();

    const repsInput = renderer!.root.findByProps({ testID: 'set-reps-workout-exercise-1-1' });
    const weightInput = renderer!.root.findByProps({ testID: 'set-weight-workout-exercise-1-1' });
    expect(repsInput.props.value).toBe('7');
    expect(weightInput.props.value).toBe('100');

    const nudgeCopy = renderer!.root.findByProps({
      testID: 'set-nudge-copy-workout-exercise-1-1',
    });
    expect(nudgeCopy.props.children).toContain('+2.5 kg');

    await ReactTestRenderer.act(async () => {
      renderer!.root.findByProps({ testID: 'set-nudge-workout-exercise-1-1' }).props.onPress();
    });

    const nudgedWeightInput = renderer!.root.findByProps({
      testID: 'set-weight-workout-exercise-1-1',
    });
    expect(nudgedWeightInput.props.value).toBe('102.5');

    await cleanup(renderer!, queryClient);
  });

  it('renders progression recommendation and shows why details for main lift', async () => {
    const queryClient = createQueryClient();

    let renderer: ReactTestRenderer.ReactTestRenderer;
    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <WorkoutRunnerScreen />
        </QueryClientProvider>,
      );
    });

    await flush();
    await flush();

    const recommendation = renderer!.root.findByProps({
      testID: 'recommended-load-workout-exercise-1',
    });
    expect(recommendation.props.children.join('')).toContain('Recommended load: 102.5 kg');

    await ReactTestRenderer.act(async () => {
      renderer!.root
        .findByProps({ testID: 'recommendation-why-toggle-workout-exercise-1' })
        .props.onPress();
    });

    const whyText = renderer!.root.findByProps({
      testID: 'recommendation-why-workout-exercise-1',
    });
    expect(whyText.props.children).toContain('increasing load');

    await cleanup(renderer!, queryClient);
  });

  it('uses adjusted session prescription as workout baseline metadata', async () => {
    const queryClient = createQueryClient();

    mockedStartWorkout.mockResolvedValueOnce({
      ...STARTED_WORKOUT,
      exercises: [
        {
          ...STARTED_WORKOUT.exercises[0],
          plannedJson: {
            ...STARTED_WORKOUT.exercises[0].plannedJson,
            sets: 4,
            reps_range: '8-10',
            rest_seconds: 150,
          },
        },
      ],
    });

    let renderer: ReactTestRenderer.ReactTestRenderer;
    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <WorkoutRunnerScreen />
        </QueryClientProvider>,
      );
    });

    await flush();
    await flush();

    const metadata = renderer!.root.findByProps({
      testID: 'exercise-meta-workout-exercise-1',
    });
    const metadataText = Array.isArray(metadata.props.children)
      ? metadata.props.children.join('')
      : String(metadata.props.children);
    expect(metadataText).toContain('2 sets');
    expect(metadataText).toContain('5-8 reps');
    expect(metadataText).toContain('Rest 120s');

    await cleanup(renderer!, queryClient);
  });

  it('loads biomechanics metadata and launches Unity preview', async () => {
    const queryClient = createQueryClient();

    let renderer: ReactTestRenderer.ReactTestRenderer;
    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <WorkoutRunnerScreen />
        </QueryClientProvider>,
      );
    });

    await flush();
    await flush();

    await ReactTestRenderer.act(async () => {
      renderer!.root.findByProps({ testID: 'anatomy-preview-workout-exercise-1' }).props.onPress();
    });

    await flush();

    expect(mockedGetExerciseBiomechanics).toHaveBeenCalledWith(
      {
        accessToken: 'access-token',
        exerciseId: 'exercise-1',
      },
      false,
    );
    expect(mockedOpenUnity).toHaveBeenCalledTimes(1);
    expect(mockedLoadExerciseBiomechanics).toHaveBeenCalledWith(BIOMECH_PREVIEW_RESPONSE);

    await cleanup(renderer!, queryClient);
  });

  it('loads substitutes and applies a local swap for the session exercise', async () => {
    const queryClient = createQueryClient();

    let renderer: ReactTestRenderer.ReactTestRenderer;
    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <WorkoutRunnerScreen />
        </QueryClientProvider>,
      );
    });

    await flush();
    await flush();

    await ReactTestRenderer.act(async () => {
      renderer!.root.findByProps({ testID: 'swap-exercise-workout-exercise-1' }).props.onPress();
    });

    await flush();

    expect(mockedGetExerciseSubstitutes).toHaveBeenCalledTimes(1);
    expect(mockedGetExerciseSubstitutes.mock.calls[0][0]).toMatchObject({
      exerciseId: 'exercise-1',
      limit: 5,
    });
    expect(mockedGetExerciseSubstitutes.mock.calls[0][1]).toBe(false);

    const whyCopy = renderer!.root.findByProps({
      testID: 'swap-option-why-workout-exercise-1-0',
    });
    expect(whyCopy.props.children).toContain('Matches squat + quads/glutes');

    await ReactTestRenderer.act(async () => {
      renderer!.root.findByProps({ testID: 'swap-option-workout-exercise-1-0' }).props.onPress();
    });

    const swappedName = renderer!.root.findByProps({ testID: 'exercise-name-workout-exercise-1' });
    expect(swappedName.props.children).toBe('Goblet Squat');

    await cleanup(renderer!, queryClient);
  });
});
