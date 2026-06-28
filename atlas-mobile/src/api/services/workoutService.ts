import type { components, operations } from '../generated/openapi';
import { atlasApiClient, getApiErrorMessage } from '../client';
import { PROGRAM_LIBRARY, WEEK_SCHEDULE_BY_PROGRAM, WORKOUT_PLAN_BY_PROGRAM } from '../mockData';

type CurrentProgramScheduleResponse = components['schemas']['CurrentProgramScheduleResponse'];
type Program = components['schemas']['Program'];
type ProgramEnrollment = components['schemas']['ProgramEnrollment'];
type ProgramSession = components['schemas']['ProgramSession'];
type ProgramSessionExercise = components['schemas']['ProgramSessionExercise'];
type ProgramWeekSchedule = components['schemas']['ProgramWeekSchedule'];
type Exercise = components['schemas']['Exercise'];
type ExerciseSubstitute = components['schemas']['ExerciseSubstitute'];
type Workout = components['schemas']['Workout'];
type WorkoutExercise = components['schemas']['WorkoutExercise'];
type WorkoutHistoryResponse = components['schemas']['WorkoutHistoryResponse'];
type WorkoutSet = components['schemas']['WorkoutSet'];
type StartWorkoutRequestBody = components['schemas']['StartWorkoutRequest'];
type CompleteWorkoutRequestBody = components['schemas']['CompleteWorkoutRequest'];

type AddWorkoutSetRequestBody =
  operations['PostWorkoutsWorkoutIdAddSet']['requestBody']['content']['application/json'];
type WorkoutsHistoryQuery = operations['GetWorkoutsHistory']['parameters']['query'];
type ExerciseSubstitutesQuery = operations['GetExerciseSubstitutesById']['parameters']['query'];

type WorkoutRequestContext = {
  accessToken: string;
};

export type WorkoutSessionPlan = {
  enrollment: ProgramEnrollment;
  program: Program;
  week: ProgramWeekSchedule;
  session: ProgramSession;
  dayLabel: string;
};

export type StartWorkoutInput = WorkoutRequestContext & {
  programSessionId?: string | null;
};

export type AddWorkoutSetLogInput = WorkoutRequestContext & {
  workoutId: string;
  workoutExerciseId: string;
  reps: number;
  weightKg: number;
  rpe?: number | null;
  idempotencyKey: string;
};

export type CompleteWorkoutInput = WorkoutRequestContext & {
  workoutId: string;
  notes?: string;
};

export type WorkoutDetailInput = WorkoutRequestContext & {
  workoutId: string;
};

export type WorkoutHistoryInput = WorkoutRequestContext & {
  limit?: number;
  cursor?: string;
};

export type ExerciseSubstitutesInput = WorkoutRequestContext & {
  exerciseId: string;
  equipment?: string[];
  injuryFlags?: string[];
  limit?: number;
};

const NETWORK_LATENCY_MS = 220;
const MOCK_CREATED_AT = '2026-01-01T00:00:00.000Z';
const DAY_LABELS = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];
const DAY_TO_INDEX: Record<string, number> = {
  Monday: 1,
  Tuesday: 2,
  Wednesday: 3,
  Thursday: 4,
  Friday: 5,
  Saturday: 6,
  Sunday: 7,
};

const MOCK_EXERCISE_CREATED_AT = '2026-01-01T00:00:00.000Z';

type MockWorkoutState = {
  activeWorkoutId: string | null;
  workoutsById: Map<string, Workout>;
  order: string[];
  setByIdempotency: Map<string, WorkoutSet>;
  sessionPlanBySessionId: Map<string, WorkoutSessionPlan>;
};

const mockWorkoutState: MockWorkoutState = {
  activeWorkoutId: null,
  workoutsById: new Map<string, Workout>(),
  order: [],
  setByIdempotency: new Map<string, WorkoutSet>(),
  sessionPlanBySessionId: new Map<string, WorkoutSessionPlan>(),
};

function sleep(ms: number): Promise<void> {
  return new Promise(resolve => {
    setTimeout(resolve, ms);
  });
}

function assertAccessToken(accessToken: string): void {
  if (!accessToken) {
    throw new Error('Missing authentication token.');
  }
}

function nowDateString(): string {
  return new Date().toISOString().slice(0, 10);
}

function isoDayOfWeek(date: Date): number {
  return ((date.getDay() + 6) % 7) + 1;
}

function dayLabel(dayOfWeek: number): string {
  if (dayOfWeek < 1 || dayOfWeek > 7) {
    return `Day ${dayOfWeek}`;
  }

  return DAY_LABELS[dayOfWeek - 1];
}

function createMockExercise(input: {
  id: string;
  slug: string;
  name: string;
  movementPattern: string;
  primaryMuscles: string[];
  secondaryMuscles: string[];
  contraindications: string[];
  equipment: string[];
  difficulty: Exercise['difficulty'];
  description: string;
}): Exercise {
  return {
    id: input.id,
    slug: input.slug,
    name: input.name,
    movementPattern: input.movementPattern,
    primaryMuscleGroup: input.primaryMuscles[0] ?? 'unknown',
    primaryMuscles: input.primaryMuscles,
    secondaryMuscles: input.secondaryMuscles,
    contraindications: input.contraindications,
    equipment: input.equipment,
    difficulty: input.difficulty,
    description: input.description,
    createdAt: MOCK_EXERCISE_CREATED_AT,
    media: [],
  };
}

function createMockSubstitute(exercise: Exercise, why: ExerciseSubstitute['why']): ExerciseSubstitute {
  return {
    exercise,
    why,
  };
}

const MOCK_SUBSTITUTES_BY_SLUG: Record<string, ExerciseSubstitute[]> = {
  'back-squat': [
    createMockSubstitute(
      createMockExercise({
        id: 'mock-exercise-goblet-squat',
        slug: 'goblet-squat',
        name: 'Goblet Squat',
        movementPattern: 'squat',
        primaryMuscles: ['quads', 'glutes'],
        secondaryMuscles: ['core'],
        contraindications: [],
        equipment: ['dumbbell', 'kettlebell'],
        difficulty: 'beginner',
        description: 'Squat variation with front-loaded implement.',
      }),
      {
        matchedPattern: ['squat'],
        matchedMuscles: ['quads', 'glutes'],
        equipmentFit: 'partial',
      },
    ),
    createMockSubstitute(
      createMockExercise({
        id: 'mock-exercise-front-squat',
        slug: 'front-squat',
        name: 'Front Squat',
        movementPattern: 'squat',
        primaryMuscles: ['quads'],
        secondaryMuscles: ['core'],
        contraindications: ['acute_knee_injury'],
        equipment: ['barbell', 'rack'],
        difficulty: 'advanced',
        description: 'Barbell front-loaded squat.',
      }),
      {
        matchedPattern: ['squat'],
        matchedMuscles: ['quads', 'core'],
        equipmentFit: 'exact',
      },
    ),
    createMockSubstitute(
      createMockExercise({
        id: 'mock-exercise-split-squat',
        slug: 'split-squat',
        name: 'Split Squat',
        movementPattern: 'squat',
        primaryMuscles: ['quads'],
        secondaryMuscles: ['glutes'],
        contraindications: [],
        equipment: ['dumbbell', 'bodyweight'],
        difficulty: 'beginner',
        description: 'Unilateral squat variation.',
      }),
      {
        matchedPattern: ['squat'],
        matchedMuscles: ['quads', 'glutes'],
        equipmentFit: 'partial',
      },
    ),
  ],
};

function normalizeTokenList(values?: string[]): string[] {
  if (!values || values.length === 0) {
    return [];
  }

  const normalized: string[] = [];
  const seen = new Set<string>();

  values.forEach(value => {
    const token = value.trim().toLowerCase();
    if (!token || seen.has(token)) {
      return;
    }

    seen.add(token);
    normalized.push(token);
  });

  return normalized;
}

function serializeTokens(values?: string[]): string | undefined {
  const normalized = normalizeTokenList(values);
  if (normalized.length === 0) {
    return undefined;
  }

  return normalized.join(',');
}

function mockExerciseSlugFromID(exerciseId: string): string {
  if (exerciseId.startsWith('mock-exercise-')) {
    return exerciseId.slice('mock-exercise-'.length);
  }

  return exerciseId;
}

function cloneWorkout(workout: Workout): Workout {
  return {
    ...workout,
    exercises: workout.exercises.map(exercise => ({
      ...exercise,
      plannedJson: { ...exercise.plannedJson },
      actualJson: { ...exercise.actualJson },
      sets: exercise.sets.map(set => ({ ...set })),
    })),
  };
}

export function selectSessionForDay(sessions: ProgramSession[], dayOfWeek: number): ProgramSession | null {
  if (sessions.length === 0) {
    return null;
  }

  const ordered = [...sessions].sort((left, right) => left.dayOfWeek - right.dayOfWeek);
  const todaysOrNext = ordered.find(session => session.dayOfWeek >= dayOfWeek);

  return todaysOrNext ?? ordered[0];
}

function buildMockProgram(): Program {
  const source = PROGRAM_LIBRARY[0];
  const schedule = source ? WEEK_SCHEDULE_BY_PROGRAM[source.id] ?? [] : [];
  const sessionDays = schedule
    .map(item => DAY_TO_INDEX[item.day] ?? 1)
    .sort((left, right) => left - right);
  const sessionCount = sessionDays.length || 3;

  if (!source) {
    return {
      id: 'mock-program',
      slug: 'mock-program',
      name: 'Mock Program',
      description: 'Mock workout plan',
      goalTags: ['general-fitness'],
      level: 'all-levels',
      weeksLength: 8,
      weeklyFrequency: 3,
      blocks: Array.from({ length: 8 }, (_, index) => ({
        id: `mock-block-mock-program-${index + 1}`,
        weekIndex: index + 1,
        sessionCount: 3,
        sessionDays: [1, 3, 5],
      })),
      createdAt: MOCK_CREATED_AT,
    };
  }

  return {
    id: source.id,
    slug: source.id,
    name: source.name,
    description: source.summary,
    goalTags: [source.goal],
    level: 'all-levels',
    weeksLength: source.durationWeeks,
    weeklyFrequency: sessionCount,
    blocks: Array.from({ length: source.durationWeeks }, (_, index) => ({
      id: `mock-block-${source.id}-${index + 1}`,
      weekIndex: index + 1,
      sessionCount,
      sessionDays,
    })),
    createdAt: MOCK_CREATED_AT,
  };
}

function buildMockWeek(program: Program): ProgramWeekSchedule {
  const schedule = WEEK_SCHEDULE_BY_PROGRAM[program.id] ?? [];

  return {
    id: `mock-week-${program.id}-1`,
    weekIndex: 1,
    sessions: schedule.map((item, index) => {
      const sessionId = `mock-session-${program.id}-${index + 1}`;
      const workoutPlan = WORKOUT_PLAN_BY_PROGRAM[program.id];
      const exercises: ProgramSessionExercise[] = (workoutPlan?.exercises ?? []).map(
        (exercise, exerciseIndex) => ({
          id: `mock-session-exercise-${sessionId}-${exerciseIndex + 1}`,
          orderIndex: exerciseIndex + 1,
          exerciseId: `mock-exercise-${exercise.id}`,
          exerciseSlug: exercise.id,
          exerciseName: exercise.name,
          prescription: {
            sets: exercise.targetSets,
            reps_range: exercise.targetReps,
            rest_seconds: exercise.restSeconds,
          },
          substitutionCandidates: [],
        }),
      );

      return {
        id: sessionId,
        dayOfWeek: DAY_TO_INDEX[item.day] ?? index + 1,
        name: item.workoutName,
        exercises,
      };
    }),
  };
}

function buildMockCurrentSchedule(): CurrentProgramScheduleResponse | null {
  const program = buildMockProgram();
  const week = buildMockWeek(program);
  if (week.sessions.length === 0) {
    return null;
  }

  const enrollment: ProgramEnrollment = {
    id: `mock-enrollment-${program.id}`,
    userId: 'mock-user',
    programId: program.id,
    startDate: nowDateString(),
    currentWeek: 1,
    createdAt: new Date().toISOString(),
  };
  const blockStart = new Date(`${enrollment.startDate}T00:00:00.000Z`);
  const blockEnd = new Date(blockStart.getTime() + 6 * 24 * 60 * 60 * 1000);

  return {
    enrollment,
    program,
    context: {
      blockWeekIndex: 1,
      templateWeekIndex: 1,
      totalWeeks: program.weeksLength,
      blockStartDate: enrollment.startDate,
      blockEndDate: blockEnd.toISOString().slice(0, 10),
    },
    week,
  };
}

function buildMockWorkoutFromSession(sessionPlan: WorkoutSessionPlan): Workout {
  return {
    id: `mock-workout-${Date.now().toString(36)}`,
    userId: 'mock-user',
    programSessionId: sessionPlan.session.id,
    startedAt: new Date().toISOString(),
    completedAt: null,
    notes: '',
    createdAt: new Date().toISOString(),
    exercises: sessionPlan.session.exercises.map(item => ({
      id: `mock-workout-exercise-${item.id}`,
      workoutId: `mock-workout-${Date.now().toString(36)}`,
      exerciseId: item.exerciseId,
      orderIndex: item.orderIndex,
      plannedJson: {
        sets: item.prescription.sets,
        reps_range: item.prescription.reps_range,
        rest_seconds: item.prescription.rest_seconds,
      },
      actualJson: {},
      createdAt: new Date().toISOString(),
      exerciseSlug: item.exerciseSlug,
      exerciseName: item.exerciseName,
      sets: [],
    })),
  };
}

function setWorkoutInMockState(workout: Workout): void {
  mockWorkoutState.workoutsById.set(workout.id, cloneWorkout(workout));
  if (!mockWorkoutState.order.includes(workout.id)) {
    mockWorkoutState.order.unshift(workout.id);
  }
}

function resolveMockSessionPlan(programSessionId?: string | null): WorkoutSessionPlan | null {
  if (programSessionId) {
    const fromMap = mockWorkoutState.sessionPlanBySessionId.get(programSessionId);
    if (fromMap) {
      return fromMap;
    }
  }

  const schedule = buildMockCurrentSchedule();
  if (!schedule) {
    return null;
  }

  const nowDay = isoDayOfWeek(new Date());
  const session = selectSessionForDay(schedule.week.sessions, nowDay);
  if (!session) {
    return null;
  }

  const selected: WorkoutSessionPlan = {
    enrollment: schedule.enrollment,
    program: schedule.program,
    week: schedule.week,
    session,
    dayLabel: dayLabel(session.dayOfWeek),
  };

  mockWorkoutState.sessionPlanBySessionId.set(session.id, selected);
  return selected;
}

export async function getCurrentSessionPlan(
  input: WorkoutRequestContext,
  useMockMode: boolean,
  options?: { now?: Date },
): Promise<WorkoutSessionPlan | null> {
  if (useMockMode) {
    await sleep(NETWORK_LATENCY_MS);

    const schedule = buildMockCurrentSchedule();
    if (!schedule) {
      return null;
    }

    const now = options?.now ?? new Date();
    const session = selectSessionForDay(schedule.week.sessions, isoDayOfWeek(now));
    if (!session) {
      return null;
    }

    const plan: WorkoutSessionPlan = {
      enrollment: schedule.enrollment,
      program: schedule.program,
      week: schedule.week,
      session,
      dayLabel: dayLabel(session.dayOfWeek),
    };

    mockWorkoutState.sessionPlanBySessionId.set(session.id, plan);
    return plan;
  }

  assertAccessToken(input.accessToken);

  const response = await atlasApiClient.GET('/api/v1/programs/current', {
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (response.data) {
    const now = options?.now ?? new Date();
    const session = selectSessionForDay(response.data.week.sessions, isoDayOfWeek(now));
    if (!session) {
      return null;
    }

    return {
      enrollment: response.data.enrollment,
      program: response.data.program,
      week: response.data.week,
      session,
      dayLabel: dayLabel(session.dayOfWeek),
    };
  }

  if (response.response.status === 404) {
    return null;
  }

  throw new Error(getApiErrorMessage(response.error, 'Unable to load current workout session plan.'));
}

export async function getWorkoutHistory(
  input: WorkoutHistoryInput,
  useMockMode: boolean,
): Promise<WorkoutHistoryResponse> {
  if (useMockMode) {
    await sleep(NETWORK_LATENCY_MS);

    const workouts = mockWorkoutState.order
      .map(id => mockWorkoutState.workoutsById.get(id))
      .filter((value): value is Workout => Boolean(value))
      .slice(0, input.limit ?? 20)
      .map(workout => ({
        id: workout.id,
        userId: workout.userId,
        programSessionId: workout.programSessionId,
        startedAt: workout.startedAt,
        completedAt: workout.completedAt,
        notes: workout.notes,
        createdAt: workout.createdAt,
      }));

    return {
      workouts,
      next_cursor: null,
    };
  }

  assertAccessToken(input.accessToken);

  const query: WorkoutsHistoryQuery = {
    limit: input.limit,
    cursor: input.cursor,
  };

  const response = await atlasApiClient.GET('/api/v1/workouts/history', {
    params: {
      query,
    },
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to load workout history.'));
  }

  return response.data;
}

export async function getWorkoutDetail(
  input: WorkoutDetailInput,
  useMockMode: boolean,
): Promise<Workout> {
  if (useMockMode) {
    await sleep(NETWORK_LATENCY_MS);

    const workout = mockWorkoutState.workoutsById.get(input.workoutId);
    if (!workout) {
      throw new Error('Workout not found.');
    }

    return cloneWorkout(workout);
  }

  assertAccessToken(input.accessToken);

  const response = await atlasApiClient.GET('/api/v1/workouts/{id}', {
    params: {
      path: {
        id: input.workoutId,
      },
    },
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to load workout detail.'));
  }

  return response.data.workout;
}

export async function getExerciseSubstitutes(
  input: ExerciseSubstitutesInput,
  useMockMode: boolean,
): Promise<ExerciseSubstitute[]> {
  if (useMockMode) {
    await sleep(NETWORK_LATENCY_MS / 2);

    const equipment = normalizeTokenList(input.equipment);
    const injuryFlags = normalizeTokenList(input.injuryFlags);
    const maxCount =
      typeof input.limit === 'number' && Number.isFinite(input.limit)
        ? Math.max(1, Math.min(20, Math.floor(input.limit)))
        : 5;

    const source = MOCK_SUBSTITUTES_BY_SLUG[mockExerciseSlugFromID(input.exerciseId)] ?? [];
    return source
      .filter(candidate => {
        const hasEquipment =
          equipment.length === 0 ||
          candidate.exercise.equipment.some(item => equipment.includes(item.toLowerCase()));
        const blockedByContra =
          injuryFlags.length > 0 &&
          candidate.exercise.contraindications.some(flag =>
            injuryFlags.includes(flag.toLowerCase()),
          );

        return hasEquipment && !blockedByContra;
      })
      .slice(0, maxCount)
      .map(item => ({
        ...item,
        exercise: {
          ...item.exercise,
          media: [...item.exercise.media],
        },
      }));
  }

  assertAccessToken(input.accessToken);

  const query: ExerciseSubstitutesQuery = {
    equipment: serializeTokens(input.equipment),
    injuryFlags: serializeTokens(input.injuryFlags),
    constraints: JSON.stringify({
      equipment: normalizeTokenList(input.equipment),
      injuryFlags: normalizeTokenList(input.injuryFlags),
    }),
  };
  if (typeof input.limit === 'number' && Number.isFinite(input.limit)) {
    query.limit = Math.max(1, Math.min(20, Math.floor(input.limit)));
  }

  const response = await atlasApiClient.GET('/api/v1/exercises/{id}/substitutes', {
    params: {
      path: {
        id: input.exerciseId,
      },
      query,
    },
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to load exercise substitutes.'));
  }

  return response.data.substitutes;
}

async function getLatestIncompleteWorkout(
  input: WorkoutRequestContext,
  useMockMode: boolean,
): Promise<Workout | null> {
  const history = await getWorkoutHistory(
    {
      accessToken: input.accessToken,
      limit: 1,
    },
    useMockMode,
  );

  const latest = history.workouts[0];
  if (!latest || latest.completedAt) {
    return null;
  }

  return getWorkoutDetail(
    {
      accessToken: input.accessToken,
      workoutId: latest.id,
    },
    useMockMode,
  );
}

export async function startWorkout(input: StartWorkoutInput, useMockMode: boolean): Promise<Workout> {
  const existing = await getLatestIncompleteWorkout(
    {
      accessToken: input.accessToken,
    },
    useMockMode,
  );
  if (existing) {
    return existing;
  }

  if (useMockMode) {
    await sleep(NETWORK_LATENCY_MS);

    const sessionPlan = resolveMockSessionPlan(input.programSessionId);
    if (!sessionPlan) {
      throw new Error('No session plan available.');
    }

    const workout = buildMockWorkoutFromSession(sessionPlan);
    const normalizedWorkout = {
      ...workout,
      exercises: workout.exercises.map(exercise => ({
        ...exercise,
        workoutId: workout.id,
      })),
    };

    mockWorkoutState.activeWorkoutId = normalizedWorkout.id;
    setWorkoutInMockState(normalizedWorkout);
    return cloneWorkout(normalizedWorkout);
  }

  assertAccessToken(input.accessToken);

  const body: StartWorkoutRequestBody | undefined = input.programSessionId
    ? {
        program_session_id: input.programSessionId,
      }
    : undefined;

  const response = await atlasApiClient.POST('/api/v1/workouts/start', {
    body,
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to start workout.'));
  }

  return response.data.workout;
}

export async function addWorkoutSetLog(
  input: AddWorkoutSetLogInput,
  useMockMode: boolean,
): Promise<WorkoutSet> {
  if (useMockMode) {
    await sleep(NETWORK_LATENCY_MS / 2);

    const workout = mockWorkoutState.workoutsById.get(input.workoutId);
    if (!workout) {
      throw new Error('Workout not found.');
    }
    if (workout.completedAt) {
      throw new Error('Workout already completed.');
    }

    const dedupeKey = `${input.workoutExerciseId}:${input.idempotencyKey}`;
    const existingSet = mockWorkoutState.setByIdempotency.get(dedupeKey);
    if (existingSet) {
      return { ...existingSet };
    }

    const exercise = workout.exercises.find(item => item.id === input.workoutExerciseId);
    if (!exercise) {
      throw new Error('Workout exercise not found.');
    }

    const setIndex = exercise.sets.length + 1;
    const set: WorkoutSet = {
      id: `mock-set-${input.workoutExerciseId}-${setIndex}`,
      workoutExerciseId: input.workoutExerciseId,
      setIndex,
      reps: input.reps,
      weightKg: input.weightKg,
      rpe: input.rpe ?? null,
      completedAt: new Date().toISOString(),
      createdAt: new Date().toISOString(),
    };

    exercise.sets = [...exercise.sets, set];
    setWorkoutInMockState(workout);
    mockWorkoutState.setByIdempotency.set(dedupeKey, set);
    return { ...set };
  }

  assertAccessToken(input.accessToken);

  const body: AddWorkoutSetRequestBody = {
    idempotency_key: input.idempotencyKey,
    workout_exercise_id: input.workoutExerciseId,
    reps: input.reps,
    weight_kg: input.weightKg,
    rpe: input.rpe ?? undefined,
  };

  const response = await atlasApiClient.POST('/api/v1/workouts/{workout_id}/add_set', {
    params: {
      path: {
        workout_id: input.workoutId,
      },
    },
    body,
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to log workout set.'));
  }

  return response.data.set;
}

export async function completeWorkout(
  input: CompleteWorkoutInput,
  useMockMode: boolean,
): Promise<Workout> {
  if (useMockMode) {
    await sleep(NETWORK_LATENCY_MS / 2);

    const workout = mockWorkoutState.workoutsById.get(input.workoutId);
    if (!workout) {
      throw new Error('Workout not found.');
    }
    if (workout.completedAt) {
      return cloneWorkout(workout);
    }

    const completed: Workout = {
      ...workout,
      completedAt: new Date().toISOString(),
      notes: input.notes?.trim() ?? workout.notes,
    };

    mockWorkoutState.activeWorkoutId = null;
    setWorkoutInMockState(completed);
    return cloneWorkout(completed);
  }

  assertAccessToken(input.accessToken);

  const body: CompleteWorkoutRequestBody | undefined = input.notes
    ? {
        notes: input.notes,
      }
    : undefined;

  const response = await atlasApiClient.POST('/api/v1/workouts/{workout_id}/complete', {
    params: {
      path: {
        workout_id: input.workoutId,
      },
    },
    body,
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to complete workout.'));
  }

  return response.data.workout;
}

export function readPlannedPrescription(exercise: WorkoutExercise): {
  sets: number;
  repsRange: string;
  restSeconds: number;
} {
  const planned = exercise.plannedJson as Record<string, unknown>;
  const setsRaw = planned.sets;
  const repsRangeRaw = planned.reps_range;
  const restSecondsRaw = planned.rest_seconds;

  const sets =
    typeof setsRaw === 'number' && Number.isFinite(setsRaw) && setsRaw > 0
      ? Math.floor(setsRaw)
      : 1;

  const repsRange =
    typeof repsRangeRaw === 'string' && repsRangeRaw.trim().length > 0
      ? repsRangeRaw
      : '-';

  const restSeconds =
    typeof restSecondsRaw === 'number' && Number.isFinite(restSecondsRaw) && restSecondsRaw > 0
      ? Math.floor(restSecondsRaw)
      : 90;

  return {
    sets,
    repsRange,
    restSeconds,
  };
}

export function injectProgressionRecommendations(
  workout: Workout,
  sessionPlan: WorkoutSessionPlan,
): Workout {
  const recommendationByOrder = new Map<
    number,
    {
      recommendedLoadKg?: number;
      progressionWhy?: string | null;
      adjustmentReasons?: string[];
      sets: number;
      repsRange: string;
      restSeconds: number;
    }
  >();

  sessionPlan.session.exercises.forEach(item => {
    const adjustmentReasons =
      item.adjustmentReasons?.map(reason => reason.trim()).filter(reason => reason.length > 0) ?? [];

    recommendationByOrder.set(item.orderIndex, {
      recommendedLoadKg: item.recommendedLoadKg ?? undefined,
      progressionWhy: item.progressionWhy,
      adjustmentReasons,
      sets: item.prescription.sets,
      repsRange: item.prescription.reps_range,
      restSeconds: item.prescription.rest_seconds,
    });
  });

  return {
    ...workout,
    exercises: workout.exercises.map(item => {
      const recommendation = recommendationByOrder.get(item.orderIndex);
      if (!recommendation) {
        return item;
      }

      const progressionWhy = recommendation.progressionWhy ?? recommendation.adjustmentReasons?.[0] ?? null;

      return {
        ...item,
        plannedJson: {
          ...item.plannedJson,
          sets: recommendation.sets,
          reps_range: recommendation.repsRange,
          rest_seconds: recommendation.restSeconds,
          recommended_load_kg: recommendation.recommendedLoadKg ?? null,
          progression_why: progressionWhy,
          adjustment_reasons:
            recommendation.adjustmentReasons && recommendation.adjustmentReasons.length > 0
              ? recommendation.adjustmentReasons
              : null,
        },
      };
    }),
  };
}

export function readProgressionRecommendation(exercise: WorkoutExercise): {
  recommendedLoadKg?: number;
  progressionWhy?: string;
  adjustmentReasons?: string[];
} {
  const planned = exercise.plannedJson as Record<string, unknown>;

  const recommendedLoadRaw = planned.recommended_load_kg ?? planned.recommendedLoadKg;
  const progressionWhyRaw = planned.progression_why ?? planned.progressionWhy;
  const adjustmentReasonsRaw = planned.adjustment_reasons ?? planned.adjustmentReasons;

  const recommendedLoadKg =
    typeof recommendedLoadRaw === 'number' && Number.isFinite(recommendedLoadRaw)
      ? recommendedLoadRaw
      : undefined;

  let progressionWhy =
    typeof progressionWhyRaw === 'string' && progressionWhyRaw.trim().length > 0
      ? progressionWhyRaw
      : undefined;

  const adjustmentReasons = Array.isArray(adjustmentReasonsRaw)
    ? adjustmentReasonsRaw
        .map(value => (typeof value === 'string' ? value.trim() : ''))
        .filter(value => value.length > 0)
    : undefined;

  if (!progressionWhy && adjustmentReasons && adjustmentReasons.length > 0) {
    progressionWhy = adjustmentReasons[0];
  }

  return {
    recommendedLoadKg,
    progressionWhy,
    adjustmentReasons,
  };
}

export type PreviousPerformanceSet = {
  setIndex: number;
  reps: number;
  weightKg: number;
  rpe?: number;
};

export type PreviousPerformance = {
  workoutId?: string;
  completedAt?: string;
  sets: PreviousPerformanceSet[];
};

function readFiniteNumber(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}

function readPositiveInteger(value: unknown): number | undefined {
  const parsed = readFiniteNumber(value);
  if (parsed === undefined || parsed < 1) {
    return undefined;
  }

  return Math.floor(parsed);
}

export function readPreviousPerformance(exercise: WorkoutExercise): PreviousPerformance | null {
  const planned = exercise.plannedJson as Record<string, unknown>;
  const previousRaw = planned.previous_performance ?? planned.previousPerformance;
  if (!previousRaw || typeof previousRaw !== 'object' || Array.isArray(previousRaw)) {
    return null;
  }

  const payload = previousRaw as Record<string, unknown>;
  const workoutIdRaw = payload.workout_id ?? payload.workoutId;
  const completedAtRaw = payload.completed_at ?? payload.completedAt;
  const setsRaw = payload.sets;

  if (!Array.isArray(setsRaw)) {
    return null;
  }

  const sets: PreviousPerformanceSet[] = [];
  for (const value of setsRaw) {
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
      continue;
    }

    const candidate = value as Record<string, unknown>;
    const setIndex = readPositiveInteger(candidate.set_index ?? candidate.setIndex);
    const reps = readPositiveInteger(candidate.reps);
    const weightKg = readFiniteNumber(candidate.weight_kg ?? candidate.weightKg);

    if (setIndex === undefined || reps === undefined || weightKg === undefined || weightKg < 0) {
      continue;
    }

    const rpe = readFiniteNumber(candidate.rpe);
    sets.push({
      setIndex,
      reps,
      weightKg,
      rpe,
    });
  }

  if (sets.length === 0) {
    return null;
  }

  sets.sort((left, right) => left.setIndex - right.setIndex);

  return {
    workoutId:
      typeof workoutIdRaw === 'string' && workoutIdRaw.trim().length > 0 ? workoutIdRaw : undefined,
    completedAt:
      typeof completedAtRaw === 'string' && completedAtRaw.trim().length > 0
        ? completedAtRaw
        : undefined,
    sets,
  };
}
