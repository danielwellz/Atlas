import React from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import ReactTestRenderer from 'react-test-renderer';
import { addWorkoutSetLog } from '../../src/api/services/workoutService';
import { createFoodLog } from '../../src/api/services/foodService';
import { setNetworkOnlineForTests } from '../../src/network/onlineManager';
import { OutboxSyncController } from '../../src/sync/OutboxSyncController';
import {
  clearOutbox,
  enqueueFoodLogOutboxItem,
  enqueueWorkoutSetOutboxItem,
  getOutboxPendingCount,
  listOutboxItems,
} from '../../src/sync/outbox';

jest.mock('../../src/api/services/workoutService', () => {
  const actual = jest.requireActual('../../src/api/services/workoutService');

  return {
    ...actual,
    addWorkoutSetLog: jest.fn(),
  };
});

jest.mock('../../src/api/services/foodService', () => {
  const actual = jest.requireActual('../../src/api/services/foodService');

  return {
    ...actual,
    createFoodLog: jest.fn(),
  };
});

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
  }),
}));

function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
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

describe('offline outbox sync integration', () => {
  const mockedAddWorkoutSetLog = addWorkoutSetLog as jest.MockedFunction<typeof addWorkoutSetLog>;
  const mockedCreateFoodLog = createFoodLog as jest.MockedFunction<typeof createFoodLog>;

  beforeEach(async () => {
    jest.clearAllMocks();
    setNetworkOnlineForTests(false);
    await clearOutbox();
  });

  it('keeps queue while offline and flushes when connectivity resumes', async () => {
    mockedAddWorkoutSetLog.mockResolvedValue({
      id: 'set-1',
      workoutExerciseId: 'exercise-1',
      setIndex: 1,
      reps: 8,
      weightKg: 80,
      rpe: null,
      completedAt: '2026-02-26T12:10:00.000Z',
      createdAt: '2026-02-26T12:10:00.000Z',
    });

    await enqueueWorkoutSetOutboxItem({
      workoutId: 'workout-1',
      workoutExerciseId: 'exercise-1',
      setIndex: 1,
      reps: 8,
      weightKg: 80,
      rpe: null,
      idempotencyKey: 'set-workout-1-exercise-1-1',
    });

    const queryClient = createQueryClient();
    let renderer: ReactTestRenderer.ReactTestRenderer;

    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <OutboxSyncController />
        </QueryClientProvider>,
      );
    });

    await flush();
    expect(mockedAddWorkoutSetLog).not.toHaveBeenCalled();
    expect(await getOutboxPendingCount()).toBe(1);

    await ReactTestRenderer.act(async () => {
      setNetworkOnlineForTests(true);
    });

    await flush();
    await flush();

    expect(mockedAddWorkoutSetLog).toHaveBeenCalledTimes(1);
    expect(mockedAddWorkoutSetLog.mock.calls[0][0].idempotencyKey).toBe(
      'set-workout-1-exercise-1-1',
    );
    expect(await getOutboxPendingCount()).toBe(0);

    await ReactTestRenderer.act(async () => {
      renderer!.unmount();
    });
    queryClient.clear();
  });

  it('retries failed sync with backoff metadata and flushes after retry window', async () => {
    setNetworkOnlineForTests(true);
    mockedAddWorkoutSetLog
      .mockRejectedValueOnce(new Error('temporary timeout'))
      .mockResolvedValueOnce({
        id: 'set-1',
        workoutExerciseId: 'exercise-1',
        setIndex: 1,
        reps: 8,
        weightKg: 80,
        rpe: null,
        completedAt: '2026-02-26T12:10:00.000Z',
        createdAt: '2026-02-26T12:10:00.000Z',
      });

    await enqueueWorkoutSetOutboxItem({
      workoutId: 'workout-1',
      workoutExerciseId: 'exercise-1',
      setIndex: 1,
      reps: 8,
      weightKg: 80,
      rpe: null,
      idempotencyKey: 'set-workout-1-exercise-1-1',
    });

    const queryClient = createQueryClient();
    let renderer: ReactTestRenderer.ReactTestRenderer;

    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <OutboxSyncController />
        </QueryClientProvider>,
      );
    });

    await flush();
    expect(mockedAddWorkoutSetLog).toHaveBeenCalledTimes(1);
    expect(await getOutboxPendingCount()).toBe(1);

    const itemsAfterFailure = await listOutboxItems();
    expect(itemsAfterFailure).toHaveLength(1);
    expect(itemsAfterFailure[0].retryCount).toBe(1);
    expect(itemsAfterFailure[0].nextRetryAt).not.toBeNull();

    const nextRetryAt = itemsAfterFailure[0].nextRetryAt!;
    const nowSpy = jest.spyOn(Date, 'now').mockReturnValue(Date.parse(nextRetryAt) + 1);
    try {
      await ReactTestRenderer.act(async () => {
        setNetworkOnlineForTests(false);
      });
      await flush();

      await ReactTestRenderer.act(async () => {
        setNetworkOnlineForTests(true);
      });
      await flush();
    } finally {
      nowSpy.mockRestore();
    }

    expect(mockedAddWorkoutSetLog).toHaveBeenCalledTimes(2);
    expect(await getOutboxPendingCount()).toBe(0);

    await ReactTestRenderer.act(async () => {
      renderer!.unmount();
    });
    queryClient.clear();
  });

  it('flushes queued food logs when connectivity resumes', async () => {
    mockedCreateFoodLog.mockResolvedValue({
      id: 'log-1',
      userId: 'user-1',
      datetime: '2026-02-26T12:10:00.000Z',
      foodId: 'food-1',
      quantity: 1.25,
      unit: 'serving',
      nutrientsSnapshot: {
        calories_kcal: 275,
        protein_g: 25,
        carbs_g: 22,
        fat_g: 9,
      },
      createdAt: '2026-02-26T12:10:00.000Z',
      food: {
        id: 'food-1',
        externalId: '012345678905',
        provider: 'edamam',
        label: 'Protein Bar',
        brand: 'Atlas Nutrition',
        nutrients: {
          calories_kcal: 220,
          protein_g: 20,
          carbs_g: 18,
          fat_g: 7,
        },
        createdAt: '2026-02-26T12:00:00.000Z',
        updatedAt: '2026-02-26T12:00:00.000Z',
      },
    });

    await enqueueFoodLogOutboxItem({
      idempotencyKey: 'food-log-2026-02-26-food-1-1',
      foodId: 'food-1',
      quantity: 1.25,
      unit: 'serving',
      datetime: '2026-02-26T12:10:00.000Z',
    });

    const queryClient = createQueryClient();
    let renderer: ReactTestRenderer.ReactTestRenderer;

    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <OutboxSyncController />
        </QueryClientProvider>,
      );
    });

    await flush();
    expect(mockedCreateFoodLog).not.toHaveBeenCalled();
    expect(await getOutboxPendingCount()).toBe(1);

    await ReactTestRenderer.act(async () => {
      setNetworkOnlineForTests(true);
    });

    await flush();
    await flush();

    expect(mockedCreateFoodLog).toHaveBeenCalledTimes(1);
    expect(mockedCreateFoodLog).toHaveBeenCalledWith({
      accessToken: 'access-token',
      foodId: 'food-1',
      quantity: 1.25,
      unit: 'serving',
      datetime: '2026-02-26T12:10:00.000Z',
    });
    expect(await getOutboxPendingCount()).toBe(0);

    await ReactTestRenderer.act(async () => {
      renderer!.unmount();
    });
    queryClient.clear();
  });
});
