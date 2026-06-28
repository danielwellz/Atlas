import type { ProductEventName, ProductEventProperties } from './types';
import { isNetworkOnline } from '../network/onlineManager';
import { NonRetryableEventError, sendAppEvent } from '../api/services/eventsService';
import { hasAnalyticsConsent } from '../api/services/consentService';
import { enqueueAnalyticsEventOutboxItem } from '../sync/outbox';

function createEventKey(eventName: string, eventTime: string): string {
  const randomSuffix = Math.random().toString(36).slice(2, 8);
  return `analytics-${eventName}-${eventTime}-${randomSuffix}`;
}

type TrackProductEventInput = {
  accessToken?: string;
  eventName: ProductEventName;
  consentGranted?: boolean;
  properties?: ProductEventProperties;
  eventTime?: string;
  useMockMode?: boolean;
};

export async function trackProductEvent(input: TrackProductEventInput): Promise<void> {
  if (input.useMockMode) {
    return;
  }

  const consentGranted = input.accessToken
    ? await hasAnalyticsConsent(input.accessToken, Boolean(input.useMockMode)).catch(() => false)
    : Boolean(input.consentGranted);
  if (!consentGranted) {
    return;
  }

  const eventTime = input.eventTime ?? new Date().toISOString();

  if (!isNetworkOnline()) {
    await enqueueAnalyticsEventOutboxItem({
      idempotencyKey: createEventKey(input.eventName, eventTime),
      eventName: input.eventName,
      eventTime,
      consentGranted,
      properties: input.properties,
    });
    return;
  }

  try {
    await sendAppEvent({
      accessToken: input.accessToken,
      eventName: input.eventName,
      eventTime,
      consentGranted,
      properties: input.properties,
    });
  } catch (error) {
    if (error instanceof NonRetryableEventError) {
      return;
    }

    await enqueueAnalyticsEventOutboxItem({
      idempotencyKey: createEventKey(input.eventName, eventTime),
      eventName: input.eventName,
      eventTime,
      consentGranted,
      properties: input.properties,
    });
  }
}
