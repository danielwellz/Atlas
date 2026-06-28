import { trackProductEvent } from '../../src/analytics/eventClient';
import { clearOutbox, getOutboxPendingCount, listOutboxItems } from '../../src/sync/outbox';
import { setNetworkOnlineForTests } from '../../src/network/onlineManager';
import { hasAnalyticsConsent } from '../../src/api/services/consentService';
import { NonRetryableEventError, sendAppEvent } from '../../src/api/services/eventsService';

jest.mock('../../src/api/services/consentService', () => {
  const actual = jest.requireActual('../../src/api/services/consentService');

  return {
    ...actual,
    hasAnalyticsConsent: jest.fn(),
  };
});

jest.mock('../../src/api/services/eventsService', () => {
  const actual = jest.requireActual('../../src/api/services/eventsService');

  return {
    ...actual,
    sendAppEvent: jest.fn(),
  };
});

describe('event client', () => {
  const mockedHasAnalyticsConsent = hasAnalyticsConsent as jest.MockedFunction<
    typeof hasAnalyticsConsent
  >;
  const mockedSendAppEvent = sendAppEvent as jest.MockedFunction<typeof sendAppEvent>;

  beforeEach(async () => {
    jest.clearAllMocks();
    setNetworkOnlineForTests(true);
    mockedHasAnalyticsConsent.mockResolvedValue(true);
    mockedSendAppEvent.mockResolvedValue({ accepted: true });
    await clearOutbox();
  });

  it('queues analytics events while offline', async () => {
    setNetworkOnlineForTests(false);

    await trackProductEvent({
      accessToken: 'access-token',
      eventName: 'onboarding_completed',
      useMockMode: false,
      properties: {
        goal: 'Build strength',
        days_per_week: 4,
        equipment_count: 2,
        source: 'schedule_screen',
        platform: 'ios',
        app_version: '0.0.1',
      },
    });

    expect(await getOutboxPendingCount()).toBe(1);
    const items = await listOutboxItems();
    expect(items[0].kind).toBe('analytics_event');
  });

  it('does not enqueue non-retryable failures', async () => {
    mockedSendAppEvent.mockRejectedValue(new NonRetryableEventError('consent denied'));

    await trackProductEvent({
      accessToken: 'access-token',
      eventName: 'workout_completed',
      useMockMode: false,
      properties: {
        workout_id: 'workout-1',
        duration_minutes: 40,
        exercise_count: 5,
        set_count: 18,
        completion_source: 'workout_runner',
        platform: 'ios',
        app_version: '0.0.1',
      },
    });

    expect(await getOutboxPendingCount()).toBe(0);
  });

  it('skips event submission when analytics consent is not granted', async () => {
    mockedHasAnalyticsConsent.mockResolvedValue(false);

    await trackProductEvent({
      accessToken: 'access-token',
      eventName: 'onboarding_started',
      useMockMode: false,
      properties: {
        entry_point: 'goals_screen',
        platform: 'ios',
        app_version: '0.0.1',
      },
    });

    expect(mockedSendAppEvent).not.toHaveBeenCalled();
    expect(await getOutboxPendingCount()).toBe(0);
  });
});
