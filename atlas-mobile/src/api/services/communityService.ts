import type { components, operations } from '../generated/openapi';
import { atlasApiClient, getApiErrorMessage } from '../client';

type CrewsResponse = operations['GetCrews']['responses'][200]['content']['application/json'];
type CrewResponse = operations['GetCrewsId']['responses'][200]['content']['application/json'];
type CreateCrewRequest = operations['PostCrews']['requestBody']['content']['application/json'];
type CreateCrewResponse = operations['PostCrews']['responses'][201]['content']['application/json'];
type CreateCrewInviteRequest = NonNullable<
  operations['PostCrewsIdInvites']['requestBody']
>['content']['application/json'];
type CreateCrewInviteResponse =
  operations['PostCrewsIdInvites']['responses'][201]['content']['application/json'];
type JoinCrewRequest = operations['PostCrewsJoin']['requestBody']['content']['application/json'];
type JoinCrewResponse = operations['PostCrewsJoin']['responses'][200]['content']['application/json'];
type CoachSessionsResponse =
  operations['GetCoachSessions']['responses'][200]['content']['application/json'];
type CoachSessionResponse =
  operations['GetCoachSessionsId']['responses'][200]['content']['application/json'];

type CommunityServiceContext = {
  accessToken: string;
};

export type Crew = components['schemas']['Crew'];
export type CrewMember = components['schemas']['CrewMember'];
export type CoachSessionSummary = components['schemas']['CoachSessionSummary'];
export type CoachSession = components['schemas']['CoachSession'];
export type CrewInvite = components['schemas']['CrewInvite'];

export type CreateCrewInput = CommunityServiceContext & {
  name: string;
  description?: string;
  sharedPlanUrl?: string;
  sharedHabitsUrl?: string;
  isPrivate?: boolean;
};

export type CreateCrewInviteInput = CommunityServiceContext & {
  crewId: string;
  maxUses?: number;
  expiresAt?: string;
};

export type JoinCrewByInviteInput = CommunityServiceContext & {
  inviteCode: string;
};

export type GetCrewInput = CommunityServiceContext & {
  crewId: string;
};

export type GetCoachSessionInput = CommunityServiceContext & {
  sessionId: string;
};

function assertAccessToken(accessToken: string): void {
  if (!accessToken) {
    throw new Error('Missing authentication token.');
  }
}

export async function listCrews(input: CommunityServiceContext): Promise<Crew[]> {
  assertAccessToken(input.accessToken);

  const response = await atlasApiClient.GET('/api/v1/crews', {
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to load crews.'));
  }

  const payload: CrewsResponse = response.data;
  return payload.crews;
}

export async function createCrew(input: CreateCrewInput): Promise<CreateCrewResponse> {
  assertAccessToken(input.accessToken);

  const body: CreateCrewRequest = {
    name: input.name,
    description: input.description ?? '',
    isPrivate: input.isPrivate ?? true,
  };
  if (input.sharedPlanUrl) {
    body.sharedPlanUrl = input.sharedPlanUrl;
  }
  if (input.sharedHabitsUrl) {
    body.sharedHabitsUrl = input.sharedHabitsUrl;
  }

  const response = await atlasApiClient.POST('/api/v1/crews', {
    body,
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to create crew.'));
  }

  const payload: CreateCrewResponse = response.data;
  return payload;
}

export async function getCrew(input: GetCrewInput): Promise<CrewResponse> {
  assertAccessToken(input.accessToken);

  const response = await atlasApiClient.GET('/api/v1/crews/{id}', {
    params: {
      path: {
        id: input.crewId,
      },
    },
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to load crew details.'));
  }

  const payload: CrewResponse = response.data;
  return payload;
}

export async function createCrewInvite(input: CreateCrewInviteInput): Promise<CrewInvite> {
  assertAccessToken(input.accessToken);

  const body: CreateCrewInviteRequest = {
    maxUses: input.maxUses ?? 1,
  };
  if (input.expiresAt) {
    body.expiresAt = input.expiresAt;
  }

  const response = await atlasApiClient.POST('/api/v1/crews/{id}/invites', {
    params: {
      path: {
        id: input.crewId,
      },
    },
    body,
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to create crew invite.'));
  }

  const payload: CreateCrewInviteResponse = response.data;
  return payload.invite;
}

export async function joinCrewByInvite(input: JoinCrewByInviteInput): Promise<JoinCrewResponse> {
  assertAccessToken(input.accessToken);

  const body: JoinCrewRequest = {
    inviteCode: input.inviteCode,
  };

  const response = await atlasApiClient.POST('/api/v1/crews/join', {
    body,
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to join crew.'));
  }

  const payload: JoinCrewResponse = response.data;
  return payload;
}

export async function listCoachSessions(input: CommunityServiceContext): Promise<CoachSessionSummary[]> {
  assertAccessToken(input.accessToken);

  const response = await atlasApiClient.GET('/api/v1/coach-sessions', {
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to load coach sessions.'));
  }

  const payload: CoachSessionsResponse = response.data;
  return payload.sessions;
}

export async function getCoachSessionById(input: GetCoachSessionInput): Promise<CoachSession> {
  assertAccessToken(input.accessToken);

  const response = await atlasApiClient.GET('/api/v1/coach-sessions/{id}', {
    params: {
      path: {
        id: input.sessionId,
      },
    },
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to load coach session.'));
  }

  const payload: CoachSessionResponse = response.data;
  return payload.session;
}
