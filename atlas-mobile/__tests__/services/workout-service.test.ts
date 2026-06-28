import type { components } from '../../src/api/generated/openapi';
import {
  addWorkoutSetLog,
  getExerciseSubstitutes,
  getCurrentSessionPlan,
  injectProgressionRecommendations,
  readPlannedPrescription,
  readPreviousPerformance,
  readProgressionRecommendation,
  selectSessionForDay,
} from '../../src/api/services/workoutService';

const mockGet = jest.fn();
const mockPost = jest.fn();

jest.mock('../../src/api/client', () => ({
  atlasApiClient: {
    GET: (...args: unknown[]) => mockGet(...args),
    POST: (...args: unknown[]) => mockPost(...args),
  },
  getApiErrorMessage: (_error: unknown, fallback: string) => fallback,
}));

type CurrentProgramScheduleResponse = components['schemas']['CurrentProgramScheduleResponse'];

function buildScheduleResponse(): CurrentProgramScheduleResponse {
  return {
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
      weeklyFrequency: 2,
      blocks: [
        {
          id: '7a121212-1212-4212-8212-121212121212',
          weekIndex: 1,
          sessionCount: 2,
          sessionDays: [2, 5],
        },
      ],
      createdAt: '2026-01-01T00:00:00.000Z',
    },
    context: {
      blockWeekIndex: 1,
      templateWeekIndex: 1,
      totalWeeks: 8,
      blockStartDate: '2026-02-24',
      blockEndDate: '2026-03-01',
    },
    week: {
      id: '7a444444-4444-4444-8444-444444444444',
      weekIndex: 1,
      sessions: [
        {
          id: '7a555555-5555-4555-8555-555555555555',
          dayOfWeek: 2,
          name: 'Tuesday Session',
          exercises: [],
        },
        {
          id: '7a666666-6666-4666-8666-666666666666',
          dayOfWeek: 5,
          name: 'Friday Session',
          exercises: [],
        },
      ],
    },
  };
}

describe('workoutService', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('selects today session when available and wraps to first when needed', () => {
    const sessions = buildScheduleResponse().week.sessions;

    const sameDay = selectSessionForDay(sessions, 5);
    expect(sameDay?.name).toBe('Friday Session');

    const wrapped = selectSessionForDay(sessions, 7);
    expect(wrapped?.name).toBe('Tuesday Session');
  });

  it('gets current session plan and picks the next scheduled day', async () => {
    mockGet.mockResolvedValueOnce({
      data: buildScheduleResponse(),
      response: {
        status: 200,
      },
      error: undefined,
    });

    const plan = await getCurrentSessionPlan(
      {
        accessToken: 'access-token',
      },
      false,
      {
        now: new Date('2026-02-26T12:00:00.000Z'),
      },
    );

    expect(plan?.session.name).toBe('Friday Session');
    expect(plan?.dayLabel).toBe('Friday');
    expect(mockGet).toHaveBeenCalledWith('/api/v1/programs/current', {
      headers: {
        Authorization: 'Bearer access-token',
      },
    });
  });

  it('sends idempotency key when logging a set', async () => {
    mockPost.mockResolvedValueOnce({
      data: {
        set: {
          id: '7a777777-7777-4777-8777-777777777777',
          workoutExerciseId: '7a888888-8888-4888-8888-888888888888',
          setIndex: 1,
          reps: 8,
          weightKg: 80,
          rpe: 8,
          completedAt: '2026-02-26T12:00:00.000Z',
          createdAt: '2026-02-26T12:00:00.000Z',
        },
      },
      response: {
        status: 201,
      },
      error: undefined,
    });

    const result = await addWorkoutSetLog(
      {
        accessToken: 'access-token',
        workoutId: '7a999999-9999-4999-8999-999999999999',
        workoutExerciseId: '7a888888-8888-4888-8888-888888888888',
        reps: 8,
        weightKg: 80,
        idempotencyKey: 'set-abc-123',
      },
      false,
    );

    expect(result.setIndex).toBe(1);
    expect(mockPost).toHaveBeenCalledWith('/api/v1/workouts/{workout_id}/add_set', {
      params: {
        path: {
          workout_id: '7a999999-9999-4999-8999-999999999999',
        },
      },
      body: {
        idempotency_key: 'set-abc-123',
        workout_exercise_id: '7a888888-8888-4888-8888-888888888888',
        reps: 8,
        weight_kg: 80,
        rpe: undefined,
      },
      headers: {
        Authorization: 'Bearer access-token',
      },
    });
  });

  it('parses previous performance payload from plannedJson', () => {
    const parsed = readPreviousPerformance({
      id: 'workout-exercise-1',
      workoutId: 'workout-1',
      exerciseId: 'exercise-1',
      orderIndex: 1,
      plannedJson: {
        previous_performance: {
          workout_id: 'workout-prev',
          completed_at: '2026-02-24T12:00:00.000Z',
          sets: [
            {
              set_index: 1,
              reps: 8,
              weight_kg: 100,
              rpe: 8,
            },
            {
              set_index: 2,
              reps: 6,
              weight_kg: 102.5,
            },
          ],
        },
      },
      actualJson: {},
      createdAt: '2026-02-26T12:00:00.000Z',
      exerciseSlug: 'back-squat',
      exerciseName: 'Back Squat',
      sets: [],
    });

    expect(parsed?.workoutId).toBe('workout-prev');
    expect(parsed?.completedAt).toBe('2026-02-24T12:00:00.000Z');
    expect(parsed?.sets).toEqual([
      {
        setIndex: 1,
        reps: 8,
        weightKg: 100,
        rpe: 8,
      },
      {
        setIndex: 2,
        reps: 6,
        weightKg: 102.5,
        rpe: undefined,
      },
    ]);
  });

  it('serializes substitute constraints and returns substitute payload', async () => {
    mockGet.mockResolvedValueOnce({
      data: {
        substitutes: [
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
        ],
      },
      response: {
        status: 200,
      },
      error: undefined,
    });

    const substitutes = await getExerciseSubstitutes(
      {
        accessToken: 'access-token',
        exerciseId: 'exercise-1',
        equipment: ['Dumbbell', 'Bench'],
        injuryFlags: ['acute_knee_injury'],
        limit: 3,
      },
      false,
    );

    expect(substitutes).toHaveLength(1);
    expect(substitutes[0].exercise.slug).toBe('goblet-squat');
    expect(mockGet).toHaveBeenCalledWith('/api/v1/exercises/{id}/substitutes', {
      params: {
        path: {
          id: 'exercise-1',
        },
        query: {
          constraints:
            '{"equipment":["dumbbell","bench"],"injuryFlags":["acute_knee_injury"]}',
          equipment: 'dumbbell,bench',
          injuryFlags: 'acute_knee_injury',
          limit: 3,
        },
      },
      headers: {
        Authorization: 'Bearer access-token',
      },
    });
  });

  it('uses adjusted session prescription as workout baseline when injecting progression data', () => {
    const workout: components['schemas']['Workout'] = {
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
            sets: 4,
            reps_range: '8-10',
            rest_seconds: 150,
          },
          actualJson: {},
          createdAt: '2026-02-26T12:00:00.000Z',
          exerciseSlug: 'back-squat',
          exerciseName: 'Back Squat',
          sets: [],
        },
      ],
    };

    const sessionPlan = {
      enrollment: buildScheduleResponse().enrollment,
      program: buildScheduleResponse().program,
      week: buildScheduleResponse().week,
      dayLabel: 'Friday',
      session: {
        id: 'session-1',
        dayOfWeek: 5,
        name: 'Friday Session',
        exercises: [
          {
            id: 'session-exercise-1',
            orderIndex: 1,
            exerciseId: 'exercise-1',
            exerciseSlug: 'back-squat',
            exerciseName: 'Back Squat',
            recommendedLoadKg: 102.5,
            adjustmentReasons: ['Completed 2/3 sessions last week (67% adherence).'],
            prescription: {
              sets: 2,
              reps_range: '5-8',
              rest_seconds: 120,
            },
            substitutionCandidates: [],
          },
        ],
      },
    };

    const injected = injectProgressionRecommendations(workout, sessionPlan);
    const adjustedExercise = injected.exercises[0];
    const adjustedPrescription = readPlannedPrescription(adjustedExercise);
    const progression = readProgressionRecommendation(adjustedExercise);

    expect(adjustedPrescription).toEqual({
      sets: 2,
      repsRange: '5-8',
      restSeconds: 120,
    });
    expect(progression.recommendedLoadKg).toBe(102.5);
    expect(progression.adjustmentReasons).toEqual([
      'Completed 2/3 sessions last week (67% adherence).',
    ]);
  });
});
