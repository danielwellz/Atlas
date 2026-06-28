import type { components, operations } from '../generated/openapi';
import { normalizeUTCDateKey } from '../dateKey';
import { atlasApiClient, getApiErrorMessage } from '../client';

type HabitsResponse = operations['GetHabits']['responses'][200]['content']['application/json'];
type GetHabitsQuery = operations['GetHabits']['parameters']['query'];
type HabitToggleTodayResponse =
  operations['PostHabitsIdToggleToday']['responses'][200]['content']['application/json'];
type MomentumSprintStatusResponse =
  operations['GetMomentumSprintStatus']['responses'][200]['content']['application/json'];
type MomentumSprintEnrollBody =
  operations['PostMomentumSprintEnroll']['requestBody']['content']['application/json'];
type MomentumSprintDayCompleteBody =
  operations['PostMomentumSprintDayComplete']['requestBody']['content']['application/json'];

export type Habit = components['schemas']['Habit'];
export type HabitDailyLog = components['schemas']['HabitDailyLog'];
export type MomentumSprintStatus = components['schemas']['MomentumSprintStatusResponse'];

type HabitServiceContext = {
  accessToken: string;
};

export type ListHabitsInput = HabitServiceContext & {
  dateKey: string;
};

export type ToggleHabitDailyLogInput = HabitServiceContext & {
  habitId: string;
};

export type EnrollMomentumSprintInput = HabitServiceContext & {
  goal: string;
};

export type CompleteMomentumSprintChecklistEntryInput = HabitServiceContext & {
  habitKey: string;
  completed: boolean;
  dateKey?: string;
};

function assertAccessToken(accessToken: string): void {
  if (!accessToken) {
    throw new Error('Missing authentication token.');
  }
}

export async function listHabits(input: ListHabitsInput): Promise<Habit[]> {
  assertAccessToken(input.accessToken);

  const query: GetHabitsQuery = {
    date: normalizeUTCDateKey(input.dateKey),
  };

  const response = await atlasApiClient.GET('/api/v1/habits', {
    params: {
      query,
    },
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to load habits.'));
  }

  const payload: HabitsResponse = response.data;
  return payload.habits;
}

export async function toggleHabitDailyLog(input: ToggleHabitDailyLogInput): Promise<HabitDailyLog> {
  assertAccessToken(input.accessToken);

  const response = await atlasApiClient.POST('/api/v1/habits/{id}/toggle_today', {
    params: {
      path: {
        id: input.habitId,
      },
    },
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to update habit completion.'));
  }

  const payload: HabitToggleTodayResponse = response.data;
  return payload.log;
}

export async function getMomentumSprintStatus(input: HabitServiceContext): Promise<MomentumSprintStatus> {
  assertAccessToken(input.accessToken);

  const response = await atlasApiClient.GET('/api/v1/momentum-sprint/status', {
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to load momentum sprint status.'));
  }

  const payload: MomentumSprintStatusResponse = response.data;
  return payload;
}

export async function enrollMomentumSprint(input: EnrollMomentumSprintInput): Promise<MomentumSprintStatus> {
  assertAccessToken(input.accessToken);

  const body: MomentumSprintEnrollBody = {
    goal: input.goal,
  };

  const response = await atlasApiClient.POST('/api/v1/momentum-sprint/enroll', {
    body,
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to enroll momentum sprint.'));
  }

  return response.data;
}

export async function completeMomentumSprintChecklistEntry(
  input: CompleteMomentumSprintChecklistEntryInput,
): Promise<MomentumSprintStatus> {
  assertAccessToken(input.accessToken);

  const body: MomentumSprintDayCompleteBody = {
    habitKey: input.habitKey,
    completed: input.completed,
    date: input.dateKey ? normalizeUTCDateKey(input.dateKey) : undefined,
  };

  const response = await atlasApiClient.POST('/api/v1/momentum-sprint/day-complete', {
    body,
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to update momentum sprint day.'));
  }

  return response.data;
}
