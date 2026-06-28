import React, { useMemo } from 'react';
import { ActivityIndicator, ScrollView, StyleSheet, Text, View } from 'react-native';
import { useRoute } from '@react-navigation/native';
import type { RouteProp } from '@react-navigation/native';
import { useCoachSessionDetailQuery } from '../../features/community/hooks';
import type { MainTabParamList } from '../../navigation/types';
import { Card } from '../../ui';

function formatCueTime(ms: number): string {
  const totalSeconds = Math.max(0, Math.floor(ms / 1000));
  const minutes = Math.floor(totalSeconds / 60)
    .toString()
    .padStart(2, '0');
  const seconds = (totalSeconds % 60).toString().padStart(2, '0');
  return `${minutes}:${seconds}`;
}

export function CoachSessionPlayerScreen(): React.JSX.Element {
  const route = useRoute<RouteProp<MainTabParamList, 'CoachSessionPlayer'>>();
  const sessionId = route.params?.sessionId ?? null;
  const sessionQuery = useCoachSessionDetailQuery(sessionId);

  const videoAsset = useMemo(
    () => sessionQuery.data?.assets.find(asset => asset.assetType === 'video') ?? null,
    [sessionQuery.data],
  );

  if (sessionQuery.isLoading) {
    return (
      <View style={styles.loading} testID="coach-session-loading">
        <ActivityIndicator size="large" color="#0f766e" />
      </View>
    );
  }

  if (sessionQuery.isError || !sessionQuery.data) {
    return (
      <View style={styles.loading}>
        <Text style={styles.error}>Unable to load coach session.</Text>
      </View>
    );
  }

  const session = sessionQuery.data;

  return (
    <ScrollView contentContainerStyle={styles.container} testID="coach-session-player-screen">
      <Text style={styles.title}>{session.title}</Text>
      <Text style={styles.subtitle}>
        {session.coachName} • {Math.max(1, Math.round(session.durationSeconds / 60))} min
      </Text>

      <Card>
        <Text style={styles.sectionTitle}>Video</Text>
        <Text style={styles.meta} testID="coach-session-video-url">
          {videoAsset ? videoAsset.signedUrl : 'No video asset available'}
        </Text>
      </Card>

      <Card>
        <Text style={styles.sectionTitle}>Cue Timeline</Text>
        {session.cues.length === 0 ? (
          <Text style={styles.meta}>No cues available.</Text>
        ) : (
          session.cues.map((cue, index) => (
            <View key={cue.id} style={styles.cueRow} testID={`coach-session-cue-${index}`}>
              <Text style={styles.cueTime}>
                {formatCueTime(cue.startMs)} - {formatCueTime(cue.endMs)}
              </Text>
              <Text style={styles.cueText}>{cue.cueText}</Text>
              <Text style={styles.meta}>
                {cue.biomechanicsDefinitionType}: {cue.biomechanicsDefinitionKey}
              </Text>
            </View>
          ))
        )}
      </Card>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: {
    padding: 16,
    gap: 14,
    backgroundColor: '#f8fafc',
  },
  loading: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: '#f8fafc',
  },
  title: {
    fontSize: 24,
    fontWeight: '700',
    color: '#0f172a',
  },
  subtitle: {
    color: '#334155',
    fontSize: 14,
  },
  sectionTitle: {
    fontSize: 18,
    fontWeight: '700',
    color: '#0f172a',
  },
  cueRow: {
    gap: 4,
    paddingBottom: 8,
    borderBottomWidth: 1,
    borderBottomColor: '#e2e8f0',
  },
  cueTime: {
    color: '#0f172a',
    fontWeight: '700',
    fontSize: 13,
  },
  cueText: {
    color: '#334155',
    fontSize: 14,
  },
  meta: {
    color: '#64748b',
    fontSize: 12,
  },
  error: {
    color: '#dc2626',
    fontSize: 14,
  },
});
