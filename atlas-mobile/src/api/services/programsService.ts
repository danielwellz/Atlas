import type { components, operations } from '../generated/openapi';
import { atlasApiClient, getApiErrorMessage } from '../client';
import { PROGRAM_LIBRARY, WEEK_SCHEDULE_BY_PROGRAM } from '../mockData';

type ProgramTemplate = components['schemas']['Program'];
type ProgramEnrollment = components['schemas']['ProgramEnrollment'];
type CurrentProgramScheduleResponse = components['schemas']['CurrentProgramScheduleResponse'];
type CurrentProgramSessionsResponse = components['schemas']['CurrentProgramSessionsResponse'];
type ProgramEnrollRequest =
  operations['PostProgramsEnroll']['requestBody']['content']['application/json'];
type ProgramCurrentSessionsQuery =
  operations['GetProgramsCurrentSessions']['parameters']['query'];
type ProgramsResponse = operations['GetPrograms']['responses'][200]['content']['application/json'];
type ProgramEnrollmentResponse =
  operations['PostProgramsEnroll']['responses'][200]['content']['application/json'];

type ProgramsRequestContext = {
  accessToken: string;
};

type EnrollInProgramInput = ProgramsRequestContext & {
  programId: string;
};

type ProgramSessionsWindowInput = ProgramsRequestContext & {
  from: string;
  to: string;
};

const NETWORK_LATENCY_MS = 220;
const MOCK_CREATED_AT = '2026-01-01T00:00:00.000Z';
const DAY_TO_INDEX: Record<string, number> = {
  Monday: 1,
  Tuesday: 2,
  Wednesday: 3,
  Thursday: 4,
  Friday: 5,
  Saturday: 6,
  Sunday: 7,
};

const MOCK_PROGRAMS: ProgramTemplate[] = PROGRAM_LIBRARY.map(program => ({
  id: program.id,
  slug: program.id,
  name: program.name,
  description: program.summary,
  goalTags: [program.goal],
  level: 'all-levels',
  weeksLength: program.durationWeeks,
  weeklyFrequency: (WEEK_SCHEDULE_BY_PROGRAM[program.id] ?? []).length || 3,
  blocks: Array.from({ length: program.durationWeeks }, (_, index) => ({
    id: `mock-block-${program.id}-${index + 1}`,
    weekIndex: index + 1,
    sessionCount: (WEEK_SCHEDULE_BY_PROGRAM[program.id] ?? []).length || 3,
    sessionDays: (WEEK_SCHEDULE_BY_PROGRAM[program.id] ?? [])
      .map(item => DAY_TO_INDEX[item.day] ?? 1)
      .sort((left, right) => left - right),
  })),
  createdAt: MOCK_CREATED_AT,
}));

let mockEnrollmentProgramId: string | null = MOCK_PROGRAMS[0]?.id ?? null;

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

function parseDateOnly(value: string): Date {
  return new Date(`${value}T00:00:00.000Z`);
}

function toDateOnly(value: Date): string {
  return value.toISOString().slice(0, 10);
}

function isoDayOfWeek(value: Date): number {
  return ((value.getUTCDay() + 6) % 7) + 1;
}

function createMockEnrollment(programId: string): ProgramEnrollment {
  return {
    id: `mock-enrollment-${programId}`,
    userId: 'mock-user',
    programId,
    startDate: nowDateString(),
    currentWeek: 1,
    createdAt: new Date().toISOString(),
  };
}

function normalizeExerciseSlug(value: string): string {
  const slug = value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');

  return slug || 'session-focus';
}

function buildMockCurrentWeekResponse(
  program: ProgramTemplate,
  enrollment: ProgramEnrollment,
): CurrentProgramScheduleResponse | null {
  const scheduleItems = WEEK_SCHEDULE_BY_PROGRAM[program.id] ?? [];
  if (scheduleItems.length === 0) {
    return null;
  }
  const blockStart = parseDateOnly(enrollment.startDate);
  const blockEnd = new Date(blockStart.getTime() + 6 * 24 * 60 * 60 * 1000);

  return {
    enrollment,
    program,
    context: {
      blockWeekIndex: 1,
      templateWeekIndex: 1,
      totalWeeks: program.weeksLength,
      blockStartDate: enrollment.startDate,
      blockEndDate: toDateOnly(blockEnd),
    },
    week: {
      id: `mock-week-${program.id}-1`,
      weekIndex: 1,
      sessions: scheduleItems.map((item, index) => ({
        id: `mock-session-${program.id}-${index + 1}`,
        dayOfWeek: DAY_TO_INDEX[item.day] ?? index + 1,
        name: item.workoutName,
        exercises: [
          {
            id: `mock-session-exercise-${program.id}-${index + 1}`,
            orderIndex: 1,
            exerciseId: `mock-exercise-${program.id}-${index + 1}`,
            exerciseSlug: normalizeExerciseSlug(item.focus),
            exerciseName: item.focus,
            prescription: {
              sets: 3,
              reps_range: '8-12',
              rest_seconds: 90,
            },
            substitutionCandidates: [],
          },
        ],
      })),
    },
  };
}

function resolveMockBlockID(program: ProgramTemplate, blockWeekIndex: number): string {
  const block = program.blocks.find(item => item.weekIndex === blockWeekIndex);
  return block?.id ?? `mock-block-${program.id}-${blockWeekIndex}`;
}

function buildMockScheduledSessions(
  schedule: CurrentProgramScheduleResponse,
  from: string,
  to: string,
): CurrentProgramSessionsResponse {
  const fromDate = parseDateOnly(from);
  const toDate = parseDateOnly(to);
  const enrollmentStart = parseDateOnly(schedule.enrollment.startDate);
  const sessions: CurrentProgramSessionsResponse['sessions'] = [];

  for (
    let cursor = new Date(fromDate.getTime());
    cursor.getTime() <= toDate.getTime();
    cursor = new Date(cursor.getTime() + 24 * 60 * 60 * 1000)
  ) {
    if (cursor.getTime() < enrollmentStart.getTime()) {
      continue;
    }

    const elapsedDays = Math.floor((cursor.getTime() - enrollmentStart.getTime()) / (24 * 60 * 60 * 1000));
    const blockWeekIndex = Math.min(
      schedule.program.weeksLength,
      Math.max(1, Math.floor(elapsedDays / 7) + 1),
    );
    const dayOfWeek = isoDayOfWeek(cursor);
    const matchingSession = schedule.week.sessions.find(item => item.dayOfWeek === dayOfWeek);
    if (!matchingSession) {
      continue;
    }

    sessions.push({
      programSessionId: matchingSession.id,
      blockId: resolveMockBlockID(schedule.program, blockWeekIndex),
      blockWeekIndex,
      scheduledDate: toDateOnly(cursor),
      dayOfWeek,
      name: matchingSession.name,
      exercises: matchingSession.exercises,
    });
  }

  return {
    enrollment: schedule.enrollment,
    program: schedule.program,
    context: schedule.context,
    sessions,
  };
}

export async function listPrograms(useMockMode: boolean): Promise<ProgramTemplate[]> {
  if (useMockMode) {
    await sleep(NETWORK_LATENCY_MS);
    return MOCK_PROGRAMS;
  }

  const response = await atlasApiClient.GET('/api/v1/programs');

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to load programs.'));
  }

  const payload: ProgramsResponse = response.data;
  return payload.programs;
}

export async function enrollInProgram(
  input: EnrollInProgramInput,
  useMockMode: boolean,
): Promise<ProgramEnrollment> {
  if (useMockMode) {
    await sleep(NETWORK_LATENCY_MS);

    const exists = MOCK_PROGRAMS.some(program => program.id === input.programId);
    if (!exists) {
      throw new Error('Program not found.');
    }

    mockEnrollmentProgramId = input.programId;
    return createMockEnrollment(input.programId);
  }

  assertAccessToken(input.accessToken);

  const body: ProgramEnrollRequest = {
    program_id: input.programId,
  };

  const response = await atlasApiClient.POST('/api/v1/programs/enroll', {
    body,
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to enroll in program.'));
  }

  const payload: ProgramEnrollmentResponse = response.data;
  return payload.enrollment;
}

export async function fetchCurrentWeekSchedule(
  input: ProgramsRequestContext,
  useMockMode: boolean,
): Promise<CurrentProgramScheduleResponse | null> {
  if (useMockMode) {
    await sleep(NETWORK_LATENCY_MS);

    if (!mockEnrollmentProgramId) {
      return null;
    }

    const program = MOCK_PROGRAMS.find(item => item.id === mockEnrollmentProgramId);
    if (!program) {
      return null;
    }

    const enrollment = createMockEnrollment(program.id);
    return buildMockCurrentWeekResponse(program, enrollment);
  }

  assertAccessToken(input.accessToken);

  const response = await atlasApiClient.GET('/api/v1/programs/current', {
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (response.data) {
    return response.data;
  }

  if (response.response.status === 404) {
    return null;
  }

  throw new Error(getApiErrorMessage(response.error, 'Unable to load current week schedule.'));
}

export async function fetchCurrentProgramSessions(
  input: ProgramSessionsWindowInput,
  useMockMode: boolean,
): Promise<CurrentProgramSessionsResponse | null> {
  if (useMockMode) {
    await sleep(NETWORK_LATENCY_MS);

    if (!mockEnrollmentProgramId) {
      return null;
    }

    const program = MOCK_PROGRAMS.find(item => item.id === mockEnrollmentProgramId);
    if (!program) {
      return null;
    }

    const enrollment = createMockEnrollment(program.id);
    const schedule = buildMockCurrentWeekResponse(program, enrollment);
    if (!schedule) {
      return null;
    }

    return buildMockScheduledSessions(schedule, input.from, input.to);
  }

  assertAccessToken(input.accessToken);

  const query: ProgramCurrentSessionsQuery = {
    from: input.from,
    to: input.to,
  };

  const response = await atlasApiClient.GET('/api/v1/programs/current/sessions', {
    params: {
      query,
    },
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (response.data) {
    return response.data;
  }

  if (response.response.status === 404) {
    return null;
  }

  throw new Error(getApiErrorMessage(response.error, 'Unable to load scheduled sessions.'));
}
