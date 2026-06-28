import { DeviceEventEmitter, NativeModules } from 'react-native';
import {
  closeUnity,
  openUnity,
  receiveMessageFromUnity,
  receiveUnityState,
  sendMessageToUnity,
} from '../src/native/unityBridge';

describe('unityBridge', () => {
  const openUnityMock = jest.fn(async () => undefined);
  const closeUnityMock = jest.fn(async () => undefined);
  const sendMessageToUnityMock = jest.fn(async () => undefined);
  const receiveMessageFromUnityMock = jest.fn(async () => true);

  beforeEach(() => {
    (NativeModules as Record<string, unknown>).UnityBridgeModule = {
      openUnity: openUnityMock,
      closeUnity: closeUnityMock,
      sendMessageToUnity: sendMessageToUnityMock,
      receiveMessageFromUnity: receiveMessageFromUnityMock,
    };
  });

  afterEach(() => {
    DeviceEventEmitter.removeAllListeners('UnityMessage');
    jest.clearAllMocks();
  });

  it('forwards bridge calls and receives Unity events', async () => {
    await Promise.all([openUnity(), openUnity()]);
    await sendMessageToUnity('anatomy.ping', '{"source":"jest"}');
    await Promise.all([closeUnity(), closeUnity()]);

    expect(openUnityMock).toHaveBeenCalledTimes(1);
    expect(sendMessageToUnityMock).toHaveBeenCalledWith('anatomy.ping', '{"source":"jest"}');
    expect(closeUnityMock).toHaveBeenCalledTimes(1);

    const callback = jest.fn();
    const unsubscribe = receiveMessageFromUnity(callback);

    expect(receiveMessageFromUnityMock).toHaveBeenCalledTimes(1);

    DeviceEventEmitter.emit('UnityMessage', {
      topic: 'unity.ready',
      payload: '{"status":"ok"}',
    });

    expect(callback).toHaveBeenCalledWith({
      topic: 'unity.ready',
      payload: '{"status":"ok"}',
    });

    unsubscribe();
  });

  it('parses unity.state payloads for loaded and failed callbacks', () => {
    const callback = jest.fn();
    const unsubscribe = receiveUnityState(callback);

    DeviceEventEmitter.emit('UnityMessage', {
      topic: 'unity.state',
      payload: '{"state":"loaded","mode":"unity","reason":"","openCount":2,"closeCount":1}',
    });
    DeviceEventEmitter.emit('UnityMessage', {
      topic: 'unity.state',
      payload:
        '{"state":"failed","mode":"fallback","reason":"unity_runtime_unavailable","openCount":2,"closeCount":1}',
    });

    expect(callback).toHaveBeenNthCalledWith(1, {
      state: 'loaded',
      mode: 'unity',
      reason: '',
      openCount: 2,
      closeCount: 1,
      rawPayload: '{"state":"loaded","mode":"unity","reason":"","openCount":2,"closeCount":1}',
    });
    expect(callback).toHaveBeenNthCalledWith(2, {
      state: 'failed',
      mode: 'fallback',
      reason: 'unity_runtime_unavailable',
      openCount: 2,
      closeCount: 1,
      rawPayload:
        '{"state":"failed","mode":"fallback","reason":"unity_runtime_unavailable","openCount":2,"closeCount":1}',
    });

    unsubscribe();
  });
});
