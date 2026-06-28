import type { components, operations } from '../generated/openapi';
import { atlasApiClient, getApiErrorMessage } from '../client';

type DashboardSummaryResponse =
  operations['GetDashboardSummary']['responses'][200]['content']['application/json'];

export type DashboardSummary = components['schemas']['DashboardSummary'];

export type DashboardServiceContext = {
  accessToken: string;
};

function assertAccessToken(accessToken: string): void {
  if (!accessToken) {
    throw new Error('Missing authentication token.');
  }
}

export async function getDashboardSummary(
  input: DashboardServiceContext,
): Promise<DashboardSummary> {
  assertAccessToken(input.accessToken);

  const response = await atlasApiClient.GET('/api/v1/dashboard/summary', {
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to load dashboard summary.'));
  }

  const payload: DashboardSummaryResponse = response.data;
  return payload.summary;
}

