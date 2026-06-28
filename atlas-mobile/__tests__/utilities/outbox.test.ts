import AsyncStorage from '@react-native-async-storage/async-storage';
import {
  clearOutbox,
  dequeueOutboxItem,
  enqueueAnalyticsEventOutboxItem,
  enqueueFoodLogOutboxItem,
  enqueueNutritionOutboxItem,
  enqueueWorkoutSetOutboxItem,
  flushOutbox,
  getOutboxPendingCount,
  listOutboxItems,
} from '../../src/sync/outbox';
import { addWorkoutSetLog } from '../../src/api/services/workoutService';
import { createFoodLog } from '../../src/api/services/foodService';
import { setNetworkOnlineForTests } from '../../src/network/onlineManager';

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

describe('outbox utility', () => {
  const mockedAddWorkoutSetLog = addWorkoutSetLog as jest.MockedFunction<typeof addWorkoutSetLog>;
  const mockedCreateFoodLog = createFoodLog as jest.MockedFunction<typeof createFoodLog>;

  beforeEach(async () => {
    jest.clearAllMocks();
    setNetworkOnlineForTests(true);
    await clearOutbox();
  });

  it('enqueues unique items and dedupes repeated idempotency keys', async () => {
    const firstInsert = await enqueueWorkoutSetOutboxItem({
      workoutId: 'workout-1',
      workoutExerciseId: 'exercise-1',
      setIndex: 1,
      reps: 8,
      weightKg: 80,
      rpe: null,
      idempotencyKey: 'set-1',
    });

    const duplicateInsert = await enqueueWorkoutSetOutboxItem({
      workoutId: 'workout-1',
      workoutExerciseId: 'exercise-1',
      setIndex: 1,
      reps: 8,
      weightKg: 80,
      rpe: null,
      idempotencyKey: 'set-1',
    });

    const nutritionInsert = await enqueueNutritionOutboxItem({
      idempotencyKey: 'nutrition-2026-02-26',
      dateKey: '2026-02-26',
      caloriesEstimate: 2100,
      proteinGEstimate: 170,
      notes: 'offline',
    });
    const foodLogInsert = await enqueueFoodLogOutboxItem({
      idempotencyKey: 'food-log-2026-02-26-food-1-1',
      foodId: 'food-1',
      quantity: 1.5,
      unit: 'serving',
      datetime: '2026-02-26T12:00:00.000Z',
    });
    const analyticsInsert = await enqueueAnalyticsEventOutboxItem({
      idempotencyKey: 'analytics-onboarding-completed',
      eventName: 'onboarding_completed',
      eventTime: '2026-02-26T12:00:00.000Z',
      consentGranted: true,
      properties: {
        goal: 'Build strength',
        days_per_week: 4,
        equipment_count: 2,
        source: 'schedule_screen',
        platform: 'ios',
        app_version: '0.0.1',
      },
    });

    expect(firstInsert).toBe(true);
    expect(duplicateInsert).toBe(false);
    expect(nutritionInsert).toBe(true);
    expect(foodLogInsert).toBe(true);
    expect(analyticsInsert).toBe(true);
    expect(await getOutboxPendingCount()).toBe(4);
  });

  it('dequeues an item by idempotency key', async () => {
    await enqueueWorkoutSetOutboxItem({
      workoutId: 'workout-1',
      workoutExerciseId: 'exercise-1',
      setIndex: 1,
      reps: 8,
      weightKg: 80,
      rpe: null,
      idempotencyKey: 'set-1',
    });
    await enqueueNutritionOutboxItem({
      idempotencyKey: 'nutrition-2026-02-26',
      dateKey: '2026-02-26',
    });
    await enqueueAnalyticsEventOutboxItem({
      idempotencyKey: 'analytics-workout-completed',
      eventName: 'workout_completed',
      eventTime: '2026-02-26T12:05:00.000Z',
      consentGranted: true,
      properties: {
        workout_id: 'workout-1',
        duration_minutes: 36,
        exercise_count: 5,
        set_count: 16,
        completion_source: 'workout_runner',
        platform: 'ios',
        app_version: '0.0.1',
      },
    });

    await dequeueOutboxItem('set-1');
    const items = await listOutboxItems();

    expect(items).toHaveLength(2);
    expect(items[0].idempotencyKey).toBe('nutrition-2026-02-26');
    expect(items[1].idempotencyKey).toBe('analytics-workout-completed');
  });

  it('hydrates legacy outbox items without retry metadata', async () => {
    await AsyncStorage.setItem(
      'atlas.mobile.sync.outbox.v1',
      JSON.stringify([
        {
          kind: 'workout_set',
          idempotencyKey: 'legacy-set-1',
          createdAt: '2026-02-26T12:00:00.000Z',
          payload: {
            workoutId: 'workout-1',
            workoutExerciseId: 'exercise-1',
            setIndex: 1,
            reps: 8,
            weightKg: 80,
            rpe: null,
            idempotencyKey: 'legacy-set-1',
          },
        },
      ]),
    );

    const items = await listOutboxItems();
    expect(items).toHaveLength(1);
    expect(items[0].retryCount).toBe(0);
    expect(items[0].nextRetryAt).toBeNull();
  });

  it('retries failed workout sync with backoff and keeps a stable idempotency key', async () => {
    mockedAddWorkoutSetLog
      .mockRejectedValueOnce(new Error('temporary timeout'))
      .mockResolvedValueOnce({
        id: 'set-1',
        workoutExerciseId: 'exercise-1',
        setIndex: 1,
        reps: 8,
        weightKg: 80,
        rpe: null,
        completedAt: '2026-02-26T12:05:00.000Z',
        createdAt: '2026-02-26T12:05:00.000Z',
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

    const firstFlush = await flushOutbox('access-token');
    expect(firstFlush.flushedCount).toBe(0);
    expect(firstFlush.pendingCount).toBe(1);
    expect(mockedAddWorkoutSetLog).toHaveBeenCalledTimes(1);
    expect(mockedAddWorkoutSetLog.mock.calls[0][0].idempotencyKey).toBe(
      'set-workout-1-exercise-1-1',
    );

    const afterFirstFailure = await listOutboxItems();
    expect(afterFirstFailure).toHaveLength(1);
    expect(afterFirstFailure[0].retryCount).toBe(1);
    expect(afterFirstFailure[0].nextRetryAt).not.toBeNull();

    const immediateRetryFlush = await flushOutbox('access-token');
    expect(immediateRetryFlush.flushedCount).toBe(0);
    expect(immediateRetryFlush.pendingCount).toBe(1);
    expect(mockedAddWorkoutSetLog).toHaveBeenCalledTimes(1);

    const retryAt = afterFirstFailure[0].nextRetryAt;
    expect(retryAt).not.toBeNull();

    const nowSpy = jest.spyOn(Date, 'now').mockReturnValue(Date.parse(retryAt!) + 1);
    try {
      const eventualRetryFlush = await flushOutbox('access-token');
      expect(eventualRetryFlush.flushedCount).toBe(1);
      expect(eventualRetryFlush.pendingCount).toBe(0);
    } finally {
      nowSpy.mockRestore();
    }

    expect(mockedAddWorkoutSetLog).toHaveBeenCalledTimes(2);
    expect(mockedAddWorkoutSetLog.mock.calls[1][0].idempotencyKey).toBe(
      'set-workout-1-exercise-1-1',
    );
    expect(await getOutboxPendingCount()).toBe(0);
  });

  it('flushes queued food logs with auth token', async () => {
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

    const result = await flushOutbox('access-token');
    expect(result.flushedCount).toBe(1);
    expect(result.pendingCount).toBe(0);
    expect(mockedCreateFoodLog).toHaveBeenCalledWith({
      accessToken: 'access-token',
      foodId: 'food-1',
      quantity: 1.25,
      unit: 'serving',
      datetime: '2026-02-26T12:10:00.000Z',
    });
    expect(await getOutboxPendingCount()).toBe(0);
  });
});
