import type { components, operations } from '../generated/openapi';
import { normalizeUTCDateKey } from '../dateKey';
import { atlasApiClient, getApiErrorMessage } from '../client';

type ReadinessCheckinRequest =
  operations['PostDashboardReadinessCheckin']['requestBody']['content']['application/json'];
type ReadinessCheckinResponse =
  operations['PostDashboardReadinessCheckin']['responses'][200]['content']['application/json'];

export type DashboardReadinessCheckin =
  components['schemas']['DashboardReadinessCheckin'];

type ReadinessServiceContext = {
  accessToken: string;
};

export type UpsertReadinessCheckinInput = ReadinessServiceContext & {
  dateKey?: string;
  energyLevel: number;
  sleepQuality: number;
  stressLevel: number;
};

function assertAccessToken(accessToken: string): void {
  if (!accessToken) {
    throw new Error('Missing authentication token.');
  }
}

export async function upsertDashboardReadinessCheckin(
  input: UpsertReadinessCheckinInput,
): Promise<DashboardReadinessCheckin> {
  assertAccessToken(input.accessToken);

  const body: ReadinessCheckinRequest = {
    date: normalizeUTCDateKey(input.dateKey),
    energyLevel: input.energyLevel,
    sleepQuality: input.sleepQuality,
    stressLevel: input.stressLevel,
  };

  const response = await atlasApiClient.POST(
    '/api/v1/dashboard/readiness-checkin',
    {
      body,
      headers: {
        Authorization: `Bearer ${input.accessToken}`,
      },
    },
  );

  if (!response.data) {
    throw new Error(
      getApiErrorMessage(
        response.error,
        'Unable to submit readiness check-in.',
      ),
    );
  }

  const payload: ReadinessCheckinResponse = response.data;
  return payload.checkin;
}
