import React from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import ReactTestRenderer from 'react-test-renderer';
import type { components } from '../../src/api/generated/openapi';
import {
  enrollInProgram,
  fetchCurrentProgramSessions,
  fetchCurrentWeekSchedule,
  listPrograms,
} from '../../src/api/services/programsService';
import { ProgramsScreen } from '../../src/screens/programs/ProgramsScreen';

jest.mock('../../src/api/services/programsService', () => ({
  listPrograms: jest.fn(),
  enrollInProgram: jest.fn(),
  fetchCurrentWeekSchedule: jest.fn(),
  fetchCurrentProgramSessions: jest.fn(),
}));

const mockNavigate = jest.fn();

jest.mock('@react-navigation/native', () => ({
  useNavigation: () => ({
    navigate: mockNavigate,
  }),
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
  }),
}));

jest.mock('../../src/state/MockModeContext', () => ({
  useMockMode: () => ({
    isMockMode: false,
    canUseMockMode: false,
  }),
}));

type Program = components['schemas']['Program'];
type CurrentProgramScheduleResponse = components['schemas']['CurrentProgramScheduleResponse'];
type ProgramEnrollment = components['schemas']['ProgramEnrollment'];

const PROGRAMS: Program[] = [
  {
    id: '7a111111-1111-4111-8111-111111111111',
    slug: 'hypertrophy-foundations',
    name: 'Hypertrophy Foundations',
    description: 'Build muscle with progressive overload.',
    goalTags: ['hypertrophy', 'strength'],
    level: 'beginner',
    weeksLength: 8,
    weeklyFrequency: 3,
    blocks: [
      {
        id: '7a999999-1111-4111-8111-111111111111',
        weekIndex: 1,
        sessionCount: 3,
        sessionDays: [1, 3, 5],
      },
    ],
    createdAt: '2026-01-01T00:00:00.000Z',
  },
  {
    id: '7a222222-2222-4222-8222-222222222222',
    slug: 'athletic-condition',
    name: 'Athletic Conditioning',
    description: 'Build work capacity and conditioning.',
    goalTags: ['conditioning'],
    level: 'intermediate',
    weeksLength: 6,
    weeklyFrequency: 3,
    blocks: [
      {
        id: '7a999999-2222-4222-8222-222222222222',
        weekIndex: 1,
        sessionCount: 3,
        sessionDays: [2, 4, 6],
      },
    ],
    createdAt: '2026-01-01T00:00:00.000Z',
  },
];

const BASE_ENROLLMENT: ProgramEnrollment = {
  id: '7a333333-3333-4333-8333-333333333333',
  userId: '7a444444-4444-4444-8444-444444444444',
  programId: PROGRAMS[0].id,
  startDate: '2026-02-26',
  currentWeek: 1,
  createdAt: '2026-02-26T00:00:00.000Z',
};

const CURRENT_SCHEDULE: CurrentProgramScheduleResponse = {
  enrollment: BASE_ENROLLMENT,
  program: PROGRAMS[0],
  context: {
    blockWeekIndex: 1,
    templateWeekIndex: 1,
    totalWeeks: 8,
    blockStartDate: '2026-02-24',
    blockEndDate: '2026-03-01',
  },
  week: {
    id: '7a555555-5555-4555-8555-555555555555',
    weekIndex: 1,
    sessions: [
      {
        id: '7a666666-6666-4666-8666-666666666666',
        dayOfWeek: 1,
        name: 'Lower Strength',
        exercises: [
          {
            id: '7a777777-7777-4777-8777-777777777777',
            orderIndex: 1,
            exerciseId: '7a888888-8888-4888-8888-888888888888',
            exerciseSlug: 'back-squat',
            exerciseName: 'Back Squat',
            prescription: {
              sets: 4,
              reps_range: '5-8',
              rest_seconds: 120,
            },
            adjustmentReasons: ['Completed 2/3 sessions last week (67% adherence).'],
            substitutionCandidates: [],
          },
        ],
      },
    ],
  },
};

const CURRENT_SESSIONS: components['schemas']['CurrentProgramSessionsResponse'] = {
  enrollment: BASE_ENROLLMENT,
  program: PROGRAMS[0],
  context: CURRENT_SCHEDULE.context,
  sessions: [
    {
      programSessionId: '7a666666-6666-4666-8666-666666666666',
      blockId: '7a555555-5555-4555-8555-555555555555',
      blockWeekIndex: 1,
      scheduledDate: new Date().toISOString().slice(0, 10),
      dayOfWeek: 1,
      name: 'Lower Strength',
      exercises: CURRENT_SCHEDULE.week.sessions[0].exercises,
    },
  ],
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

describe('ProgramsScreen integration', () => {
  const mockedListPrograms = listPrograms as jest.MockedFunction<typeof listPrograms>;
  const mockedEnrollInProgram = enrollInProgram as jest.MockedFunction<typeof enrollInProgram>;
  const mockedFetchCurrentWeekSchedule = fetchCurrentWeekSchedule as jest.MockedFunction<
    typeof fetchCurrentWeekSchedule
  >;
  const mockedFetchCurrentProgramSessions =
    fetchCurrentProgramSessions as jest.MockedFunction<typeof fetchCurrentProgramSessions>;

  beforeEach(() => {
    jest.clearAllMocks();
    mockNavigate.mockReset();
    mockedListPrograms.mockResolvedValue(PROGRAMS);
    mockedFetchCurrentWeekSchedule.mockResolvedValue(CURRENT_SCHEDULE);
    mockedFetchCurrentProgramSessions.mockResolvedValue(CURRENT_SESSIONS);
    mockedEnrollInProgram.mockResolvedValue(BASE_ENROLLMENT);
  });

  it('renders program cards and the current week schedule from API data', async () => {
    const queryClient = createQueryClient();

    let renderer: ReactTestRenderer.ReactTestRenderer;

    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <ProgramsScreen />
        </QueryClientProvider>,
      );
    });

    await flush();

    expect(renderer!.root.findByProps({ testID: `program-card-${PROGRAMS[0].id}` })).toBeTruthy();
    expect(renderer!.root.findByProps({ testID: `program-card-${PROGRAMS[1].id}` })).toBeTruthy();
    expect(renderer!.root.findByProps({ testID: 'current-week-schedule' })).toBeTruthy();
    expect(renderer!.root.findByProps({ testID: 'upcoming-sessions' })).toBeTruthy();

    const prescription = renderer!.root.findByProps({
      testID: `session-prescription-${CURRENT_SCHEDULE.week.sessions[0].id}`,
    });
    expect(String(prescription.props.children)).toContain('4 sets x 5-8 reps');

    const adjustment = renderer!.root.findByProps({
      testID: `session-adjustment-${CURRENT_SCHEDULE.week.sessions[0].id}`,
    });
    expect(String(adjustment.props.children)).toContain('Completed 2/3 sessions last week');

    await cleanup(renderer!, queryClient);
  });

  it('calls enroll API mutation when enroll button is pressed', async () => {
    const queryClient = createQueryClient();

    let renderer: ReactTestRenderer.ReactTestRenderer;

    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <ProgramsScreen />
        </QueryClientProvider>,
      );
    });

    await flush();

    await ReactTestRenderer.act(async () => {
      renderer!.root.findByProps({ testID: `program-enroll-${PROGRAMS[1].id}` }).props.onPress();
    });

    expect(mockedEnrollInProgram).toHaveBeenCalledWith(
      {
        accessToken: 'access-token',
        programId: PROGRAMS[1].id,
      },
      false,
    );

    await flush();
    await cleanup(renderer!, queryClient);
  });

  it("navigates to workout runner from today's scheduled session", async () => {
    const queryClient = createQueryClient();
    let renderer: ReactTestRenderer.ReactTestRenderer;

    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <ProgramsScreen />
        </QueryClientProvider>,
      );
    });

    await flush();

    await ReactTestRenderer.act(async () => {
      renderer!.root.findByProps({ testID: 'start-today-session' }).props.onPress();
    });

    expect(mockNavigate).toHaveBeenCalledWith('WorkoutRunner');

    await cleanup(renderer!, queryClient);
  });
});
