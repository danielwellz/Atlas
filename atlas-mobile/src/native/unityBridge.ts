import { DeviceEventEmitter, NativeModules } from 'react-native';

const UNITY_MESSAGE_EVENT = 'UnityMessage';
const UNITY_STATE_TOPIC = 'unity.state';

type UnityBridgeNativeModule = {
  openUnity: () => Promise<void>;
  closeUnity: () => Promise<void>;
  sendMessageToUnity: (topic: string, payload: string) => Promise<void>;
  receiveMessageFromUnity?: () => Promise<void>;
};

export type UnityMessage = {
  topic: string;
  payload: string;
};

export type UnityStateEvent = {
  state: string;
  mode: string;
  reason: string;
  openCount: number;
  closeCount: number;
  rawPayload: string;
};

let openingPromise: Promise<void> | null = null;
let closingPromise: Promise<void> | null = null;

function getNativeModule(): UnityBridgeNativeModule {
  const nativeModule = NativeModules.UnityBridgeModule as UnityBridgeNativeModule | undefined;

  if (!nativeModule) {
    throw new Error('UnityBridgeModule is not linked. Confirm iOS/Android native setup.');
  }

  return nativeModule;
}

export async function openUnity(): Promise<void> {
  if (openingPromise) {
    return openingPromise;
  }

  openingPromise = (async () => {
    if (closingPromise) {
      await closingPromise.catch(() => undefined);
    }

    await getNativeModule().openUnity();
  })().finally(() => {
    openingPromise = null;
  });

  return openingPromise;
}

export async function closeUnity(): Promise<void> {
  if (closingPromise) {
    return closingPromise;
  }

  closingPromise = getNativeModule().closeUnity().finally(() => {
    closingPromise = null;
  });

  return closingPromise;
}

export async function sendMessageToUnity(topic: string, payload: string): Promise<void> {
  await getNativeModule().sendMessageToUnity(topic, payload);
}

export function receiveMessageFromUnity(callback: (message: UnityMessage) => void): () => void {
  const nativeModule = getNativeModule();

  nativeModule.receiveMessageFromUnity?.().catch(() => {
    // Ignore optional subscription handshakes on native failures.
  });

  const subscription = DeviceEventEmitter.addListener(UNITY_MESSAGE_EVENT, event => {
    callback({
      topic: typeof event?.topic === 'string' ? event.topic : String(event?.topic ?? ''),
      payload: typeof event?.payload === 'string' ? event.payload : String(event?.payload ?? ''),
    });
  });

  return () => {
    subscription.remove();
  };
}

export function receiveUnityState(callback: (event: UnityStateEvent) => void): () => void {
  return receiveMessageFromUnity(message => {
    if (message.topic !== UNITY_STATE_TOPIC) {
      return;
    }

    const parsedPayload = parseObjectPayload(message.payload);
    callback({
      state: readStringValue(parsedPayload, 'state'),
      mode: readStringValue(parsedPayload, 'mode'),
      reason: readStringValue(parsedPayload, 'reason'),
      openCount: readNumberValue(parsedPayload, 'openCount'),
      closeCount: readNumberValue(parsedPayload, 'closeCount'),
      rawPayload: message.payload,
    });
  });
}

function parseObjectPayload(raw: string): Record<string, unknown> | null {
  try {
    const parsed = JSON.parse(raw) as unknown;
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>;
    }
  } catch {
    // Ignore malformed native payloads and fall back to defaults.
  }

  return null;
}

function readStringValue(payload: Record<string, unknown> | null, key: string): string {
  const value = payload?.[key];
  return typeof value === 'string' ? value : '';
}

function readNumberValue(payload: Record<string, unknown> | null, key: string): number {
  const value = payload?.[key];
  return typeof value === 'number' && Number.isFinite(value) ? value : 0;
}
