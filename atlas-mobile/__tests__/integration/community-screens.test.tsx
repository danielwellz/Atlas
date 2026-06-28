import React from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import ReactTestRenderer from 'react-test-renderer';
import { CoachSessionPlayerScreen } from '../../src/screens/community/CoachSessionPlayerScreen';
import { CrewScreen } from '../../src/screens/community/CrewScreen';
import {
  createCrew,
  createCrewInvite,
  getCoachSessionById,
  getCrew,
  joinCrewByInvite,
  listCoachSessions,
  listCrews,
} from '../../src/api/services/communityService';

let mockRouteParams: { sessionId: string } = {
  sessionId: 'session-1',
};

const mockNavigate = jest.fn();

jest.mock('@react-navigation/native', () => ({
  useNavigation: () => ({
    navigate: mockNavigate,
  }),
  useRoute: () => ({
    params: mockRouteParams,
  }),
}));

jest.mock('../../src/api/services/communityService', () => ({
  listCrews: jest.fn(),
  createCrew: jest.fn(),
  getCrew: jest.fn(),
  createCrewInvite: jest.fn(),
  joinCrewByInvite: jest.fn(),
  listCoachSessions: jest.fn(),
  getCoachSessionById: jest.fn(),
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
      setTimeout(() => {
        resolve();
      }, 0);
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

describe('Community screens integration', () => {
  const mockedListCrews = listCrews as jest.MockedFunction<typeof listCrews>;
  const mockedCreateCrew = createCrew as jest.MockedFunction<typeof createCrew>;
  const mockedGetCrew = getCrew as jest.MockedFunction<typeof getCrew>;
  const mockedCreateCrewInvite =
    createCrewInvite as jest.MockedFunction<typeof createCrewInvite>;
  const mockedJoinCrewByInvite =
    joinCrewByInvite as jest.MockedFunction<typeof joinCrewByInvite>;
  const mockedListCoachSessions =
    listCoachSessions as jest.MockedFunction<typeof listCoachSessions>;
  const mockedGetCoachSessionById =
    getCoachSessionById as jest.MockedFunction<typeof getCoachSessionById>;

  beforeEach(() => {
    jest.clearAllMocks();
    mockNavigate.mockReset();
    mockRouteParams = {
      sessionId: 'session-1',
    };

    mockedListCrews.mockResolvedValue([
      {
        id: 'crew-1',
        name: 'Alpha Crew',
        description: 'Private accountability crew.',
        createdByUserId: 'user-1',
        isPrivate: true,
        sharedPlanUrl: 'https://atlas.local/plans/alpha',
        sharedHabitsUrl: 'https://atlas.local/habits/alpha',
        myRole: 'owner',
        memberCount: 2,
        createdAt: '2026-02-28T10:00:00.000Z',
        updatedAt: '2026-02-28T10:00:00.000Z',
      },
    ]);

    mockedGetCrew.mockResolvedValue({
      crew: {
        id: 'crew-1',
        name: 'Alpha Crew',
        description: 'Private accountability crew.',
        createdByUserId: 'user-1',
        isPrivate: true,
        sharedPlanUrl: 'https://atlas.local/plans/alpha',
        sharedHabitsUrl: 'https://atlas.local/habits/alpha',
        myRole: 'owner',
        memberCount: 2,
        createdAt: '2026-02-28T10:00:00.000Z',
        updatedAt: '2026-02-28T10:00:00.000Z',
      },
      members: [
        {
          userId: 'user-1',
          email: 'owner@atlas.local',
          displayName: 'Owner',
          role: 'owner',
          joinedAt: '2026-02-28T10:00:00.000Z',
        },
        {
          userId: 'user-2',
          email: 'member@atlas.local',
          displayName: 'Member',
          role: 'member',
          joinedAt: '2026-02-28T10:00:00.000Z',
        },
      ],
    });

    mockedCreateCrew.mockResolvedValue({
      crew: {
        id: 'crew-created',
        name: 'Crew 120000',
        description: 'Private accountability crew.',
        createdByUserId: 'user-1',
        isPrivate: true,
        myRole: 'owner',
        memberCount: 1,
        createdAt: '2026-02-28T10:10:00.000Z',
        updatedAt: '2026-02-28T10:10:00.000Z',
      },
      members: [],
    });

    mockedCreateCrewInvite.mockResolvedValue({
      id: 'invite-1',
      crewId: 'crew-1',
      inviteCode: 'ABCD1234',
      invitedByUserId: 'user-1',
      maxUses: 5,
      usesCount: 0,
      createdAt: '2026-02-28T10:15:00.000Z',
    });

    mockedJoinCrewByInvite.mockResolvedValue({
      crew: {
        id: 'crew-1',
        name: 'Alpha Crew',
        description: 'Private accountability crew.',
        createdByUserId: 'user-1',
        isPrivate: true,
        myRole: 'member',
        memberCount: 2,
        createdAt: '2026-02-28T10:00:00.000Z',
        updatedAt: '2026-02-28T10:00:00.000Z',
      },
      joined: true,
    });

    mockedListCoachSessions.mockResolvedValue([
      {
        id: 'session-1',
        crewId: 'crew-1',
        crewName: 'Alpha Crew',
        title: 'Squat Pattern Masterclass',
        description: 'Coach-led session',
        coachName: 'Coach Elena',
        durationSeconds: 840,
        requiredTier: 'free',
        createdAt: '2026-02-28T10:20:00.000Z',
      },
    ]);

    mockedGetCoachSessionById.mockResolvedValue({
      id: 'session-1',
      crewId: 'crew-1',
      crewName: 'Alpha Crew',
      title: 'Squat Pattern Masterclass',
      description: 'Coach-led session',
      coachName: 'Coach Elena',
      durationSeconds: 840,
      requiredTier: 'free',
      createdAt: '2026-02-28T10:20:00.000Z',
      assets: [
        {
          id: 'asset-1',
          assetType: 'video',
          storageKey: 'coach/sessions/squat-masterclass.mp4',
          signedUrl: 'https://signed.example/coach/sessions/squat-masterclass.mp4',
          mimeType: 'video/mp4',
          createdAt: '2026-02-28T10:20:00.000Z',
        },
      ],
      cues: [
        {
          id: 'cue-1',
          cueIndex: 1,
          startMs: 0,
          endMs: 7000,
          cueText: 'Brace and keep chest up.',
          biomechanicsDefinitionType: 'muscle_group',
          biomechanicsDefinitionKey: 'quads',
        },
      ],
    });
  });

  it('renders crew members and navigates to coach session player', async () => {
    const queryClient = createQueryClient();
    let renderer: ReactTestRenderer.ReactTestRenderer;

    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <CrewScreen />
        </QueryClientProvider>,
      );
    });

    await flush();
    await flush();

    expect(renderer!.root.findByProps({ testID: 'crew-card-crew-1' })).toBeTruthy();
    expect(renderer!.root.findByProps({ testID: 'crew-member-user-1' })).toBeTruthy();
    expect(renderer!.root.findByProps({ testID: 'crew-shared-plan-url' })).toBeTruthy();
    expect(renderer!.root.findByProps({ testID: 'crew-shared-habits-url' })).toBeTruthy();

    await ReactTestRenderer.act(async () => {
      renderer!.root.findByProps({ testID: 'coach-session-open-session-1' }).props.onPress();
    });

    expect(mockNavigate).toHaveBeenCalledWith('CoachSessionPlayer', {
      sessionId: 'session-1',
    });

    await cleanup(renderer!, queryClient);
  });

  it('loads session detail and renders video URL with cue timeline', async () => {
    const queryClient = createQueryClient();
    let renderer: ReactTestRenderer.ReactTestRenderer;

    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <CoachSessionPlayerScreen />
        </QueryClientProvider>,
      );
    });

    await flush();
    await flush();

    const videoUrlNode = renderer!.root.findByProps({
      testID: 'coach-session-video-url',
    });
    expect(String(videoUrlNode.props.children)).toContain(
      'https://signed.example/coach/sessions/squat-masterclass.mp4',
    );
    expect(renderer!.root.findByProps({ testID: 'coach-session-cue-0' })).toBeTruthy();

    await cleanup(renderer!, queryClient);
  });

  it('routes to paywall when coach session tier is locked', async () => {
    mockedListCoachSessions.mockResolvedValue([
      {
        id: 'session-elite-1',
        crewId: 'crew-1',
        crewName: 'Alpha Crew',
        title: 'Elite Mobility Lab',
        description: 'Elite tier session',
        coachName: 'Coach Elena',
        durationSeconds: 960,
        requiredTier: 'elite',
        createdAt: '2026-02-28T10:20:00.000Z',
      },
    ]);

    const queryClient = createQueryClient();
    let renderer: ReactTestRenderer.ReactTestRenderer;

    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <CrewScreen />
        </QueryClientProvider>,
      );
    });

    await flush();
    await flush();

    await ReactTestRenderer.act(async () => {
      renderer!.root.findByProps({ testID: 'coach-session-upgrade-session-elite-1' }).props.onPress();
    });

    expect(mockNavigate).toHaveBeenCalledWith('Paywall', {
      feature: 'coach_tier_elite',
    });

    await cleanup(renderer!, queryClient);
  });
});
