import React, { useEffect, useMemo, useState } from 'react';
import { ActivityIndicator, ScrollView, StyleSheet, Text, View } from 'react-native';
import { useNavigation } from '@react-navigation/native';
import type { BottomTabNavigationProp } from '@react-navigation/bottom-tabs';
import {
  useCoachSessionsQuery,
  useCreateCrewInviteMutation,
  useCreateCrewMutation,
  useCrewDetailQuery,
  useCrewsQuery,
  useJoinCrewByInviteMutation,
} from '../../features/community/hooks';
import { canAccessCoachTier } from '../../features/entitlements';
import { useAuth } from '../../state/AuthContext';
import type { MainTabParamList } from '../../navigation/types';
import { Button, Card, Input } from '../../ui';

function formatDurationLabel(durationSeconds: number): string {
  const minutes = Math.max(1, Math.round(durationSeconds / 60));
  return `${minutes} min`;
}

export function CrewScreen(): React.JSX.Element {
  const navigation = useNavigation<BottomTabNavigationProp<MainTabParamList>>();
  const { session: authSession } = useAuth();
  const crewsQuery = useCrewsQuery();
  const createCrewMutation = useCreateCrewMutation();
  const joinCrewMutation = useJoinCrewByInviteMutation();
  const createInviteMutation = useCreateCrewInviteMutation();
  const coachSessionsQuery = useCoachSessionsQuery();

  const [selectedCrewId, setSelectedCrewId] = useState<string | null>(null);
  const [inviteCodeInput, setInviteCodeInput] = useState('');
  const [latestInviteCode, setLatestInviteCode] = useState<string | null>(null);

  useEffect(() => {
    if (!selectedCrewId && crewsQuery.data && crewsQuery.data.length > 0) {
      setSelectedCrewId(crewsQuery.data[0].id);
    }
  }, [crewsQuery.data, selectedCrewId]);

  const crewDetailQuery = useCrewDetailQuery(selectedCrewId);

  const sessionsForSelectedCrew = useMemo(() => {
    const sessions = coachSessionsQuery.data ?? [];
    if (!selectedCrewId) {
      return sessions;
    }
    return sessions.filter(session => session.crewId === selectedCrewId);
  }, [coachSessionsQuery.data, selectedCrewId]);

  const handleCreateCrew = () => {
    const suffix = new Date().toISOString().slice(11, 19).replace(/:/g, '');
    createCrewMutation.mutate(
      {
        name: `Crew ${suffix}`,
        description: 'Private accountability crew.',
        isPrivate: true,
      },
      {
        onSuccess: payload => {
          setSelectedCrewId(payload.crew.id);
        },
      },
    );
  };

  const handleJoinCrew = () => {
    const inviteCode = inviteCodeInput.trim();
    if (!inviteCode) {
      return;
    }

    joinCrewMutation.mutate(
      {
        inviteCode,
      },
      {
        onSuccess: payload => {
          setInviteCodeInput('');
          setSelectedCrewId(payload.crew.id);
        },
      },
    );
  };

  const handleCreateInvite = () => {
    if (!selectedCrewId) {
      return;
    }

    createInviteMutation.mutate(
      {
        crewId: selectedCrewId,
        maxUses: 5,
      },
      {
        onSuccess: invite => {
          setLatestInviteCode(invite.inviteCode);
        },
      },
    );
  };

  if (crewsQuery.isLoading || coachSessionsQuery.isLoading) {
    return (
      <View style={styles.loading} testID="crew-loading">
        <ActivityIndicator size="large" color="#0f766e" />
      </View>
    );
  }

  if (crewsQuery.isError) {
    return (
      <View style={styles.loading}>
        <Text style={styles.error}>Unable to load crews.</Text>
      </View>
    );
  }

  return (
    <ScrollView contentContainerStyle={styles.container} testID="crew-screen">
      <Text style={styles.title}>Crew</Text>
      <Text style={styles.subtitle}>
        Manage your private crew, invites, and coach-led sessions.
      </Text>

      <Card>
        <Text style={styles.sectionTitle}>Crew Actions</Text>
        <Button
          label="Create Private Crew"
          onPress={handleCreateCrew}
          loading={createCrewMutation.isPending}
          testID="create-crew-button"
        />
        <Input
          label="Join via Invite Code"
          placeholder="Paste invite code"
          value={inviteCodeInput}
          onChangeText={setInviteCodeInput}
          autoCapitalize="characters"
          testID="join-invite-input"
        />
        <Button
          label="Join Crew"
          variant="secondary"
          onPress={handleJoinCrew}
          loading={joinCrewMutation.isPending}
          testID="join-invite-button"
        />
        {joinCrewMutation.isError ? (
          <Text style={styles.error}>Unable to join crew with that invite code.</Text>
        ) : null}
        {createCrewMutation.isError ? (
          <Text style={styles.error}>Unable to create a crew right now.</Text>
        ) : null}
      </Card>

      <Card>
        <Text style={styles.sectionTitle}>Your Crews</Text>
        {crewsQuery.data && crewsQuery.data.length > 0 ? (
          crewsQuery.data.map(crew => (
            <View key={crew.id} style={styles.crewRow} testID={`crew-card-${crew.id}`}>
              <View style={styles.crewRowCopy}>
                <Text style={styles.crewName}>{crew.name}</Text>
                <Text style={styles.meta}>
                  {crew.memberCount} member{crew.memberCount === 1 ? '' : 's'} • {crew.myRole}
                </Text>
              </View>
              <Button
                label={selectedCrewId === crew.id ? 'Selected' : 'View'}
                variant={selectedCrewId === crew.id ? 'secondary' : 'primary'}
                onPress={() => setSelectedCrewId(crew.id)}
                disabled={selectedCrewId === crew.id}
                testID={`crew-select-${crew.id}`}
              />
            </View>
          ))
        ) : (
          <Text style={styles.meta}>No crew yet. Create one or join with an invite code.</Text>
        )}
      </Card>

      <Card>
        <Text style={styles.sectionTitle}>Crew Detail</Text>
        {crewDetailQuery.isLoading ? (
          <ActivityIndicator size="small" color="#0f766e" />
        ) : crewDetailQuery.isError || !crewDetailQuery.data ? (
          <Text style={styles.meta}>Select a crew to view members and shared links.</Text>
        ) : (
          <>
            <Text style={styles.crewName}>{crewDetailQuery.data.crew.name}</Text>
            <Text style={styles.meta}>{crewDetailQuery.data.crew.description}</Text>
            <Text style={styles.meta} testID="crew-shared-plan-url">
              Shared plan:{' '}
              {crewDetailQuery.data.crew.sharedPlanUrl
                ? crewDetailQuery.data.crew.sharedPlanUrl
                : 'Not set'}
            </Text>
            <Text style={styles.meta} testID="crew-shared-habits-url">
              Shared habits:{' '}
              {crewDetailQuery.data.crew.sharedHabitsUrl
                ? crewDetailQuery.data.crew.sharedHabitsUrl
                : 'Not set'}
            </Text>
            <Text style={styles.sectionSubTitle}>Members</Text>
            {crewDetailQuery.data.members.map(member => (
              <Text key={member.userId} style={styles.memberLine} testID={`crew-member-${member.userId}`}>
                {(member.displayName ? `${member.displayName} ` : '') + `<${member.email}>`} ({member.role})
              </Text>
            ))}
            <Button
              label="Create Invite Code"
              variant="secondary"
              onPress={handleCreateInvite}
              loading={createInviteMutation.isPending}
              testID="create-crew-invite-button"
            />
            {latestInviteCode ? (
              <Text style={styles.meta} testID="latest-crew-invite-code">
                Latest invite: {latestInviteCode}
              </Text>
            ) : null}
          </>
        )}
      </Card>

      <Card>
        <Text style={styles.sectionTitle}>Coach Sessions</Text>
        {coachSessionsQuery.isError ? (
          <Text style={styles.error}>Unable to load coach sessions.</Text>
        ) : sessionsForSelectedCrew.length === 0 ? (
          <Text style={styles.meta}>No coach session available for this crew yet.</Text>
        ) : (
          sessionsForSelectedCrew.map(session => (
            <View
              key={session.id}
              style={styles.sessionRow}
              testID={`coach-session-card-${session.id}`}>
              <View style={styles.sessionCopy}>
                <Text style={styles.sessionTitle}>{session.title}</Text>
                <Text style={styles.meta}>
                  {session.coachName} • {formatDurationLabel(session.durationSeconds)}
                </Text>
                {session.requiredTier !== 'free' ? (
                  <Text style={styles.meta}>{`Tier: ${session.requiredTier.toUpperCase()}`}</Text>
                ) : null}
              </View>
              {canAccessCoachTier(authSession?.user, session.requiredTier) ? (
                <Button
                  label="Play"
                  onPress={() =>
                    navigation.navigate('CoachSessionPlayer', {
                      sessionId: session.id,
                    })
                  }
                  testID={`coach-session-open-${session.id}`}
                />
              ) : (
                <Button
                  label="Unlock"
                  variant="secondary"
                  onPress={() =>
                    navigation.navigate('Paywall', {
                      feature: session.requiredTier === 'elite' ? 'coach_tier_elite' : 'coach_tier_pro',
                    })
                  }
                  testID={`coach-session-upgrade-${session.id}`}
                />
              )}
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
  sectionSubTitle: {
    fontSize: 14,
    fontWeight: '700',
    color: '#0f172a',
  },
  crewRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: 12,
  },
  crewRowCopy: {
    flex: 1,
    gap: 4,
  },
  crewName: {
    fontSize: 16,
    fontWeight: '700',
    color: '#0f172a',
  },
  meta: {
    color: '#475569',
    fontSize: 13,
  },
  memberLine: {
    color: '#334155',
    fontSize: 14,
  },
  sessionRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: 12,
  },
  sessionCopy: {
    flex: 1,
    gap: 4,
  },
  sessionTitle: {
    fontSize: 15,
    fontWeight: '700',
    color: '#0f172a',
  },
  error: {
    color: '#dc2626',
    fontSize: 13,
  },
});
