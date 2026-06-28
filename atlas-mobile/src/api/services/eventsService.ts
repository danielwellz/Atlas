import type { operations } from '../generated/openapi';
import { atlasApiClient, getApiErrorMessage } from '../client';

type PostEventRequestBody = operations['PostEvents']['requestBody']['content']['application/json'];
type EventIngestResponse = operations['PostEvents']['responses'][202]['content']['application/json'];

export class NonRetryableEventError extends Error {}

export type SendAppEventInput = {
  accessToken?: string;
  eventName: PostEventRequestBody['eventName'];
  eventTime: string;
  consentGranted: boolean;
  properties?: PostEventRequestBody['properties'];
};

export async function sendAppEvent(input: SendAppEventInput): Promise<EventIngestResponse> {
  const body: PostEventRequestBody = {
    eventName: input.eventName,
    eventTime: input.eventTime,
    consentGranted: input.consentGranted,
  };

  if (input.properties !== undefined) {
    body.properties = input.properties;
  }

  const response = await atlasApiClient.POST('/api/v1/events', {
    body,
    headers: input.accessToken
      ? {
          Authorization: `Bearer ${input.accessToken}`,
        }
      : undefined,
  });

  if (!response.data) {
    const message = getApiErrorMessage(response.error, 'Unable to send analytics event.');
    const statusCode = response.response?.status;

    if (statusCode === 400 || statusCode === 401 || statusCode === 403) {
      throw new NonRetryableEventError(message);
    }

    throw new Error(message);
  }

  return response.data;
}
