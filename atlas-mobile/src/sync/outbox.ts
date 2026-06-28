import AsyncStorage from '@react-native-async-storage/async-storage';
import {
  addWorkoutSetLog,
  type AddWorkoutSetLogInput,
} from '../api/services/workoutService';
import {
  createFoodLog,
  type CreateFoodLogInput,
} from '../api/services/foodService';
import {
  upsertDailyNutritionCheckin,
  type UpsertDailyNutritionCheckinInput,
} from '../api/services/nutritionService';
import {
  NonRetryableEventError,
  sendAppEvent,
} from '../api/services/eventsService';
import type { QueueableProductEvent } from '../analytics/types';
import { isNetworkOnline } from '../network/onlineManager';

const OUTBOX_STORAGE_KEY = 'atlas.mobile.sync.outbox.v1';
const OUTBOX_MAX_ITEMS = 500;
const RETRY_BASE_DELAY_MS = 1_000;
const RETRY_MAX_DELAY_MS = 60_000;

type WorkoutSetOutboxPayload = Omit<AddWorkoutSetLogInput, 'accessToken'> & {
  setIndex: number;
};

type FoodLogOutboxPayload = Omit<CreateFoodLogInput, 'accessToken'>;

type NutritionOutboxPayload = Omit<UpsertDailyNutritionCheckinInput, 'accessToken'>;

type AnalyticsEventOutboxPayload = QueueableProductEvent;

type OutboxItemBase = {
  idempotencyKey: string;
  createdAt: string;
  retryCount: number;
  nextRetryAt: string | null;
};

export type WorkoutSetOutboxItem = OutboxItemBase & {
  kind: 'workout_set';
  payload: WorkoutSetOutboxPayload;
};

export type FoodLogOutboxItem = OutboxItemBase & {
  kind: 'food_log';
  payload: FoodLogOutboxPayload;
};

export type NutritionOutboxItem = OutboxItemBase & {
  kind: 'nutrition_checkin';
  payload: NutritionOutboxPayload;
};

export type AnalyticsEventOutboxItem = OutboxItemBase & {
  kind: 'analytics_event';
  payload: AnalyticsEventOutboxPayload;
};

export type OutboxItem =
  | WorkoutSetOutboxItem
  | FoodLogOutboxItem
  | NutritionOutboxItem
  | AnalyticsEventOutboxItem;

export type EnqueueWorkoutSetOutboxItemInput = WorkoutSetOutboxPayload;

export type EnqueueFoodLogOutboxItemInput = FoodLogOutboxPayload & {
  idempotencyKey: string;
};

export type EnqueueNutritionOutboxItemInput = NutritionOutboxPayload & {
  idempotencyKey: string;
};

export type EnqueueAnalyticsEventOutboxItemInput = AnalyticsEventOutboxPayload & {
  idempotencyKey: string;
};

export type FlushOutboxResult = {
  flushedCount: number;
  pendingCount: number;
};

type OutboxListener = (items: OutboxItem[]) => void;

const listeners = new Set<OutboxListener>();
let flushInFlight: Promise<FlushOutboxResult> | null = null;

function emitOutbox(items: OutboxItem[]): void {
  for (const listener of listeners) {
    listener(items);
  }
}

function normalizeRetryCount(value: unknown): number {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) {
    return 0;
  }

  return Math.floor(value);
}

function normalizeNextRetryAt(value: unknown): string | null {
  if (typeof value !== 'string' || value.trim().length === 0) {
    return null;
  }

  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? value : null;
}

function normalizeOutboxItem(value: unknown): OutboxItem | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return null;
  }

  const candidate = value as Partial<OutboxItem> & {
    retryCount?: unknown;
    nextRetryAt?: unknown;
  };

  if (
    typeof candidate.idempotencyKey !== 'string' ||
    candidate.idempotencyKey.trim().length === 0 ||
    typeof candidate.createdAt !== 'string' ||
    candidate.createdAt.trim().length === 0 ||
    (candidate.kind !== 'workout_set' &&
      candidate.kind !== 'food_log' &&
      candidate.kind !== 'nutrition_checkin' &&
      candidate.kind !== 'analytics_event') ||
    !candidate.payload
  ) {
    return null;
  }

  return {
    ...candidate,
    retryCount: normalizeRetryCount(candidate.retryCount),
    nextRetryAt: normalizeNextRetryAt(candidate.nextRetryAt),
  } as OutboxItem;
}

async function readStoredOutbox(): Promise<OutboxItem[]> {
  const raw = await AsyncStorage.getItem(OUTBOX_STORAGE_KEY);
  if (!raw) {
    return [];
  }

  try {
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) {
      return [];
    }

    return parsed
      .map(item => normalizeOutboxItem(item))
      .filter((item): item is OutboxItem => item !== null);
  } catch {
    return [];
  }
}

async function writeStoredOutbox(items: OutboxItem[]): Promise<void> {
  await AsyncStorage.setItem(OUTBOX_STORAGE_KEY, JSON.stringify(items));
  emitOutbox(items);
}

function applyOutboxCapacity(items: OutboxItem[]): OutboxItem[] {
  if (items.length <= OUTBOX_MAX_ITEMS) {
    return items;
  }

  const trimmed = [...items];
  while (trimmed.length > OUTBOX_MAX_ITEMS) {
    const analyticsIndex = trimmed.findIndex(item => item.kind === 'analytics_event');
    const dropIndex = analyticsIndex >= 0 ? analyticsIndex : 0;
    trimmed.splice(dropIndex, 1);
  }

  return trimmed;
}

function appendWithCapacity(items: OutboxItem[], nextItem: OutboxItem): OutboxItem[] {
  return applyOutboxCapacity([...items, nextItem]);
}

export async function listOutboxItems(): Promise<OutboxItem[]> {
  return readStoredOutbox();
}

export async function getOutboxPendingCount(): Promise<number> {
  const items = await listOutboxItems();
  return items.length;
}

export function subscribeOutbox(listener: OutboxListener): () => void {
  listeners.add(listener);
  listOutboxItems()
    .then(items => {
      listener(items);
    })
    .catch(() => {});

  return () => {
    listeners.delete(listener);
  };
}

export async function clearOutbox(): Promise<void> {
  await writeStoredOutbox([]);
}

export async function dequeueOutboxItem(idempotencyKey: string): Promise<void> {
  const items = await readStoredOutbox();
  const next = items.filter(item => item.idempotencyKey !== idempotencyKey);
  await writeStoredOutbox(next);
}

export async function enqueueWorkoutSetOutboxItem(
  payload: EnqueueWorkoutSetOutboxItemInput,
): Promise<boolean> {
  const items = await readStoredOutbox();
  if (items.some(item => item.idempotencyKey === payload.idempotencyKey)) {
    return false;
  }

  const nextItem: WorkoutSetOutboxItem = {
    kind: 'workout_set',
    idempotencyKey: payload.idempotencyKey,
    createdAt: new Date().toISOString(),
    retryCount: 0,
    nextRetryAt: null,
    payload,
  };

  await writeStoredOutbox(appendWithCapacity(items, nextItem));
  return true;
}

export async function enqueueFoodLogOutboxItem(
  input: EnqueueFoodLogOutboxItemInput,
): Promise<boolean> {
  const items = await readStoredOutbox();
  if (items.some(item => item.idempotencyKey === input.idempotencyKey)) {
    return false;
  }

  const nextItem: FoodLogOutboxItem = {
    kind: 'food_log',
    idempotencyKey: input.idempotencyKey,
    createdAt: new Date().toISOString(),
    retryCount: 0,
    nextRetryAt: null,
    payload: {
      foodId: input.foodId,
      quantity: input.quantity,
      unit: input.unit,
      datetime: input.datetime,
    },
  };

  await writeStoredOutbox(appendWithCapacity(items, nextItem));
  return true;
}

export async function enqueueNutritionOutboxItem(
  input: EnqueueNutritionOutboxItemInput,
): Promise<boolean> {
  const items = await readStoredOutbox();
  if (items.some(item => item.idempotencyKey === input.idempotencyKey)) {
    return false;
  }

  const nextItem: NutritionOutboxItem = {
    kind: 'nutrition_checkin',
    idempotencyKey: input.idempotencyKey,
    createdAt: new Date().toISOString(),
    retryCount: 0,
    nextRetryAt: null,
    payload: {
      dateKey: input.dateKey,
      caloriesEstimate: input.caloriesEstimate,
      proteinGEstimate: input.proteinGEstimate,
      notes: input.notes,
    },
  };

  await writeStoredOutbox(appendWithCapacity(items, nextItem));
  return true;
}

export async function enqueueAnalyticsEventOutboxItem(
  input: EnqueueAnalyticsEventOutboxItemInput,
): Promise<boolean> {
  const items = await readStoredOutbox();
  if (items.some(item => item.idempotencyKey === input.idempotencyKey)) {
    return false;
  }

  const nextItem: AnalyticsEventOutboxItem = {
    kind: 'analytics_event',
    idempotencyKey: input.idempotencyKey,
    createdAt: new Date().toISOString(),
    retryCount: 0,
    nextRetryAt: null,
    payload: {
      eventName: input.eventName,
      eventTime: input.eventTime,
      consentGranted: input.consentGranted,
      properties: input.properties,
    },
  };

  await writeStoredOutbox(appendWithCapacity(items, nextItem));
  return true;
}

function shouldAttemptNow(item: OutboxItem, nowMs: number): boolean {
  if (!item.nextRetryAt) {
    return true;
  }

  const retryAtMs = Date.parse(item.nextRetryAt);
  if (!Number.isFinite(retryAtMs)) {
    return true;
  }

  return nowMs >= retryAtMs;
}

function computeRetryDelayMs(retryCount: number): number {
  const exponentialFactor = Math.max(0, retryCount - 1);
  return Math.min(RETRY_MAX_DELAY_MS, RETRY_BASE_DELAY_MS * 2 ** exponentialFactor);
}

function markRetryScheduled(item: OutboxItem, nowMs: number): OutboxItem {
  const nextRetryCount = item.retryCount + 1;
  const nextRetryAtMs = nowMs + computeRetryDelayMs(nextRetryCount);

  return {
    ...item,
    retryCount: nextRetryCount,
    nextRetryAt: new Date(nextRetryAtMs).toISOString(),
  };
}

async function flushOutboxInternal(accessToken?: string): Promise<FlushOutboxResult> {
  if (!isNetworkOnline()) {
    const pendingCount = await getOutboxPendingCount();
    return {
      flushedCount: 0,
      pendingCount,
    };
  }

  const items = await readStoredOutbox();
  if (items.length === 0) {
    return {
      flushedCount: 0,
      pendingCount: 0,
    };
  }

  let flushedCount = 0;
  const remaining: OutboxItem[] = [];
  let hasBlockedItem = false;
  let didMutate = false;
  const nowMs = Date.now();

  for (const item of items) {
    if (hasBlockedItem || !isNetworkOnline()) {
      remaining.push(item);
      continue;
    }

    if (!shouldAttemptNow(item, nowMs)) {
      hasBlockedItem = true;
      remaining.push(item);
      continue;
    }

    try {
      if (item.kind === 'workout_set') {
        if (!accessToken) {
          hasBlockedItem = true;
          remaining.push(item);
          continue;
        }

        await addWorkoutSetLog(
          {
            accessToken,
            workoutId: item.payload.workoutId,
            workoutExerciseId: item.payload.workoutExerciseId,
            reps: item.payload.reps,
            weightKg: item.payload.weightKg,
            rpe: item.payload.rpe,
            idempotencyKey: item.payload.idempotencyKey,
          },
          false,
        );
      } else if (item.kind === 'food_log') {
        if (!accessToken) {
          hasBlockedItem = true;
          remaining.push(item);
          continue;
        }

        await createFoodLog({
          accessToken,
          ...item.payload,
        });
      } else if (item.kind === 'nutrition_checkin') {
        if (!accessToken) {
          hasBlockedItem = true;
          remaining.push(item);
          continue;
        }

        await upsertDailyNutritionCheckin({
          accessToken,
          ...item.payload,
        });
      } else {
        await sendAppEvent({
          accessToken,
          ...item.payload,
        });
      }

      flushedCount += 1;
      didMutate = true;
    } catch (error) {
      if (error instanceof NonRetryableEventError && item.kind === 'analytics_event') {
        flushedCount += 1;
        didMutate = true;
        continue;
      }

      hasBlockedItem = true;
      remaining.push(markRetryScheduled(item, nowMs));
      didMutate = true;
    }
  }

  if (didMutate || remaining.length !== items.length) {
    await writeStoredOutbox(remaining);
  }

  return {
    flushedCount,
    pendingCount: remaining.length,
  };
}

export async function flushOutbox(accessToken?: string): Promise<FlushOutboxResult> {
  if (flushInFlight) {
    return flushInFlight;
  }

  flushInFlight = flushOutboxInternal(accessToken).finally(() => {
    flushInFlight = null;
  });

  return flushInFlight;
}
