import React, { useEffect, useState } from 'react';
import { ScrollView, StyleSheet, Text, View } from 'react-native';
import {
  setHighlightMuscles,
  setJointAngleOverlay,
  setLayerVisibility,
} from '../../native/anatomyEngineBridge';
import {
  closeUnity,
  openUnity,
  receiveMessageFromUnity,
  receiveUnityState,
  type UnityMessage,
  type UnityStateEvent,
} from '../../native/unityBridge';
import { Button, Card } from '../../ui';

const MAX_LOG_ENTRIES = 8;

function toErrorMessage(error: unknown): string {
  if (error instanceof Error && error.message) {
    return error.message;
  }

  return 'Unexpected Unity bridge error.';
}

export function AnatomyScreen(): React.JSX.Element {
  const [isOpening, setIsOpening] = useState(false);
  const [isClosing, setIsClosing] = useState(false);
  const [isUpdating, setIsUpdating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [messages, setMessages] = useState<UnityMessage[]>([]);
  const [lastUnityState, setLastUnityState] = useState<UnityStateEvent | null>(null);
  const [showMuscles, setShowMuscles] = useState(true);
  const [showSkeleton, setShowSkeleton] = useState(true);
  const [showJointAngles, setShowJointAngles] = useState(true);

  useEffect(() => {
    let unsubscribe = () => {};
    let unsubscribeState = () => {};

    try {
      unsubscribe = receiveMessageFromUnity(message => {
        setMessages(previous => [message, ...previous].slice(0, MAX_LOG_ENTRIES));
      });

      unsubscribeState = receiveUnityState(event => {
        setLastUnityState(event);
      });
    } catch (nativeError) {
      setError(toErrorMessage(nativeError));
    }

    return () => {
      unsubscribe();
      unsubscribeState();
    };
  }, []);

  const handleOpenUnity = async (): Promise<void> => {
    setIsOpening(true);
    setError(null);

    try {
      await openUnity();
    } catch (nativeError) {
      setError(toErrorMessage(nativeError));
    } finally {
      setIsOpening(false);
    }
  };

  const handleCloseUnity = async (): Promise<void> => {
    setIsClosing(true);
    setError(null);

    try {
      await closeUnity();
    } catch (nativeError) {
      setError(toErrorMessage(nativeError));
    } finally {
      setIsClosing(false);
    }
  };

  const handleLayerToggle = async (
    nextShowMuscles: boolean,
    nextShowSkeleton: boolean,
  ): Promise<void> => {
    setIsUpdating(true);
    setError(null);

    try {
      await setLayerVisibility({
        showMuscles: nextShowMuscles,
        showSkeleton: nextShowSkeleton,
      });
      setShowMuscles(nextShowMuscles);
      setShowSkeleton(nextShowSkeleton);
    } catch (nativeError) {
      setError(toErrorMessage(nativeError));
    } finally {
      setIsUpdating(false);
    }
  };

  const handleJointOverlayToggle = async (nextEnabled: boolean): Promise<void> => {
    setIsUpdating(true);
    setError(null);

    try {
      await setJointAngleOverlay({
        enabled: nextEnabled,
      });
      setShowJointAngles(nextEnabled);
    } catch (nativeError) {
      setError(toErrorMessage(nativeError));
    } finally {
      setIsUpdating(false);
    }
  };

  const handleHighlightPrimaryMuscles = async (): Promise<void> => {
    setIsUpdating(true);
    setError(null);

    try {
      await setHighlightMuscles({
        muscleGroups: ['quads', 'glutes', 'core'],
      });
    } catch (nativeError) {
      setError(toErrorMessage(nativeError));
    } finally {
      setIsUpdating(false);
    }
  };

  return (
    <ScrollView contentContainerStyle={styles.container} testID="anatomy-screen">
      <Text style={styles.title}>Anatomy</Text>
      <Text style={styles.subtitle}>Launches Unity full-screen for biomechanics and anatomy scenes.</Text>

      <Card>
        <Text style={styles.sectionTitle}>Unity Controls</Text>
        <View style={styles.actionStack}>
          <Button
            label="Open Unity"
            onPress={handleOpenUnity}
            loading={isOpening}
            testID="anatomy-open-unity"
          />
          <Button
            label={showMuscles ? 'Hide Muscles' : 'Show Muscles'}
            variant="secondary"
            onPress={() => {
              handleLayerToggle(!showMuscles, showSkeleton).catch(() => undefined);
            }}
            loading={isUpdating}
            testID="anatomy-toggle-muscles"
          />
          <Button
            label={showSkeleton ? 'Hide Skeleton' : 'Show Skeleton'}
            variant="secondary"
            onPress={() => {
              handleLayerToggle(showMuscles, !showSkeleton).catch(() => undefined);
            }}
            loading={isUpdating}
            testID="anatomy-toggle-skeleton"
          />
          <Button
            label={showJointAngles ? 'Disable Joint Angles' : 'Enable Joint Angles'}
            variant="secondary"
            onPress={() => {
              handleJointOverlayToggle(!showJointAngles).catch(() => undefined);
            }}
            loading={isUpdating}
            testID="anatomy-toggle-joint-overlay"
          />
          <Button
            label="Highlight Primary Muscles"
            variant="secondary"
            onPress={() => {
              handleHighlightPrimaryMuscles().catch(() => undefined);
            }}
            loading={isUpdating}
            testID="anatomy-highlight-primary"
          />
          <Button
            label="Close Unity"
            variant="danger"
            onPress={handleCloseUnity}
            loading={isClosing}
            testID="anatomy-close-unity"
          />
        </View>
      </Card>

      <Card>
        <Text style={styles.sectionTitle}>Bridge Log</Text>
        {lastUnityState ? (
          <View style={styles.stateRow}>
            <Text style={styles.logTopic}>State: {lastUnityState.state || 'unknown'}</Text>
            <Text style={styles.logPayload}>
              mode={lastUnityState.mode || 'n/a'} reason={lastUnityState.reason || 'n/a'}
            </Text>
          </View>
        ) : null}
        {messages.length === 0 ? (
          <Text style={styles.empty}>No Unity messages received yet.</Text>
        ) : (
          messages.map((message, index) => (
            <View key={`${message.topic}-${index}`} style={styles.logRow}>
              <Text style={styles.logTopic}>{message.topic}</Text>
              <Text style={styles.logPayload}>{message.payload}</Text>
            </View>
          ))
        )}
      </Card>

      {error ? (
        <Card>
          <Text style={styles.error}>{error}</Text>
        </Card>
      ) : null}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: {
    padding: 16,
    gap: 12,
  },
  title: {
    fontSize: 28,
    fontWeight: '700',
    color: '#0f172a',
  },
  subtitle: {
    color: '#334155',
    fontSize: 15,
  },
  sectionTitle: {
    fontSize: 18,
    fontWeight: '700',
    color: '#0f172a',
    marginBottom: 12,
  },
  actionStack: {
    gap: 10,
  },
  empty: {
    color: '#475569',
  },
  stateRow: {
    gap: 4,
  },
  logRow: {
    borderTopWidth: StyleSheet.hairlineWidth,
    borderTopColor: '#cbd5e1',
    paddingTop: 10,
    marginTop: 10,
    gap: 4,
  },
  logTopic: {
    fontSize: 14,
    fontWeight: '700',
    color: '#0f172a',
  },
  logPayload: {
    fontSize: 13,
    color: '#1e293b',
  },
  error: {
    color: '#b91c1c',
    fontWeight: '600',
  },
});
