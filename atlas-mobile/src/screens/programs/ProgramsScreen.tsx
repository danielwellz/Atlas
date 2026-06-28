import React, { useMemo } from 'react';
import { ActivityIndicator, ScrollView, StyleSheet, Text, View } from 'react-native';
import { useNavigation } from '@react-navigation/native';
import type { BottomTabNavigationProp } from '@react-navigation/bottom-tabs';
import type { components } from '../../api/generated/openapi';
import {
  useCurrentProgramSessionsQuery,
  useCurrentWeekScheduleQuery,
  useEnrollProgramMutation,
  useProgramsQuery,
} from '../../features/programs/hooks';
import type { MainTabParamList } from '../../navigation/types';
import { useAuth } from '../../state/AuthContext';
import { Button, Card } from '../../ui';

const DAY_LABELS = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];
type ProgramSessionExercise = components['schemas']['ProgramSessionExercise'];

function dayLabel(dayOfWeek: number): string {
  if (dayOfWeek >= 1 && dayOfWeek <= 7) {
    return DAY_LABELS[dayOfWeek - 1];
  }

  return `Day ${dayOfWeek}`;
}

function summarizeExercises(names: string[]): string {
  if (names.length === 0) {
    return 'No exercises';
  }

  if (names.length <= 2) {
    return names.join(', ');
  }

  return `${names.slice(0, 2).join(', ')} +${names.length - 2} more`;
}

function dateRangeWindow(daysAhead: number): { from: string; to: string } {
  const today = new Date();
  const start = new Date(Date.UTC(today.getUTCFullYear(), today.getUTCMonth(), today.getUTCDate()));
  const end = new Date(start.getTime() + daysAhead * 24 * 60 * 60 * 1000);

  return {
    from: start.toISOString().slice(0, 10),
    to: end.toISOString().slice(0, 10),
  };
}

function formatDateLabel(dateKey: string): string {
  const parsed = new Date(`${dateKey}T00:00:00.000Z`);
  if (Number.isNaN(parsed.getTime())) {
    return dateKey;
  }
  return parsed.toISOString().slice(5, 10);
}

function formatPrescriptionSummary(exercise: ProgramSessionExercise | undefined): string {
  if (!exercise) {
    return 'Prescription not available';
  }

  return `${exercise.prescription.sets} sets x ${exercise.prescription.reps_range} reps • Rest ${exercise.prescription.rest_seconds}s`;
}

function collectAdjustmentReasons(exercises: ProgramSessionExercise[]): string[] {
  const reasons: string[] = [];
  const seen = new Set<string>();

  exercises.forEach(exercise => {
    const candidates = [
      ...(exercise.adjustmentReasons ?? []),
      ...(exercise.progressionWhy ? [exercise.progressionWhy] : []),
    ];

    candidates.forEach(candidate => {
      const trimmed = candidate.trim();
      if (!trimmed) {
        return;
      }

      const normalized = trimmed.toLowerCase();
      if (seen.has(normalized)) {
        return;
      }

      seen.add(normalized);
      reasons.push(trimmed);
    });
  });

  return reasons;
}

export function ProgramsScreen(): React.JSX.Element {
  const navigation = useNavigation<BottomTabNavigationProp<MainTabParamList>>();
  const { session } = useAuth();
  const programsQuery = useProgramsQuery();
  const scheduleQuery = useCurrentWeekScheduleQuery();
  const { from, to } = useMemo(() => dateRangeWindow(13), []);
  const sessionsQuery = useCurrentProgramSessionsQuery(from, to);
  const enrollMutation = useEnrollProgramMutation();
  const todayDate = new Date().toISOString().slice(0, 10);

  if (!session) {
    return (
      <View style={styles.loading} testID="programs-no-session">
        <Text style={styles.error}>You must be logged in to view programs.</Text>
      </View>
    );
  }

  if (programsQuery.isLoading || scheduleQuery.isLoading || sessionsQuery.isLoading) {
    return (
      <View style={styles.loading} testID="programs-loading">
        <ActivityIndicator size="large" color="#0f766e" />
      </View>
    );
  }

  if (programsQuery.isError || !programsQuery.data) {
    return (
      <View style={styles.loading}>
        <Text style={styles.error}>Unable to load programs.</Text>
      </View>
    );
  }

  const currentSchedule = scheduleQuery.data;
  const scheduledSessions = sessionsQuery.data?.sessions ?? [];
  const todaysSession = scheduledSessions.find(item => item.scheduledDate === todayDate);
  const enrolledProgramId = currentSchedule?.enrollment.programId;
  const activeEnrollProgramId = enrollMutation.isPending ? enrollMutation.variables : undefined;

  return (
    <ScrollView contentContainerStyle={styles.container} testID="programs-screen">
      <Text style={styles.title}>Programs</Text>
      <Text style={styles.subtitle}>Enroll and preview your current training week.</Text>

      <View style={styles.programsList}>
        {programsQuery.data.map(program => (
          <Card key={program.id} testID={`program-card-${program.id}`}>
            <Text style={styles.programName}>{program.name}</Text>
            <Text style={styles.meta}>Level: {program.level}</Text>
            <Text style={styles.meta}>Length: {program.weeksLength} weeks</Text>
            <Text style={styles.summary}>{program.description}</Text>
            <Text style={styles.meta}>Goals: {program.goalTags.join(', ') || 'General fitness'}</Text>
            <Button
              label={enrolledProgramId === program.id ? 'Enrolled' : 'Enroll'}
              variant={enrolledProgramId === program.id ? 'secondary' : 'primary'}
              disabled={enrolledProgramId === program.id || enrollMutation.isPending}
              loading={enrollMutation.isPending && activeEnrollProgramId === program.id}
              onPress={() => enrollMutation.mutate(program.id)}
              testID={`program-enroll-${program.id}`}
            />
          </Card>
        ))}
      </View>

      <Card testID="current-week-schedule">
        <Text style={styles.sectionTitle}>Current Week Schedule</Text>
        {scheduleQuery.isError ? (
          <Text style={styles.error}>Unable to load current week schedule.</Text>
        ) : !currentSchedule ? (
          <Text style={styles.meta}>No active schedule yet.</Text>
        ) : (
          <>
            <Text style={styles.meta}>
              {currentSchedule.program.name} • Block {currentSchedule.context.blockWeekIndex}/
              {currentSchedule.context.totalWeeks} • Template Week {currentSchedule.context.templateWeekIndex}
            </Text>
            {currentSchedule.week.sessions.map(programSession => {
              const adjustmentReasons = collectAdjustmentReasons(programSession.exercises);

              return (
                <View key={programSession.id}>
                  <View style={styles.scheduleRow}>
                    <Text style={styles.day}>{dayLabel(programSession.dayOfWeek)}</Text>
                    <View style={styles.scheduleCopy}>
                      <Text style={styles.workoutName}>{programSession.name}</Text>
                      <Text style={styles.meta}>
                        {programSession.exercises.length} exercise
                        {programSession.exercises.length === 1 ? '' : 's'} •{' '}
                        {summarizeExercises(programSession.exercises.map(item => item.exerciseName))}
                      </Text>
                      <Text
                        style={styles.meta}
                        testID={`session-prescription-${programSession.id}`}>
                        {formatPrescriptionSummary(programSession.exercises[0])}
                      </Text>
                      {adjustmentReasons.length > 0 ? (
                        <Text
                          style={styles.adjustmentReason}
                          testID={`session-adjustment-${programSession.id}`}>
                          Adjustment: {adjustmentReasons.join(' ')}
                        </Text>
                      ) : null}
                    </View>
                  </View>
                </View>
              );
            })}
          </>
        )}
      </Card>

      <Card testID="upcoming-sessions">
        <Text style={styles.sectionTitle}>Upcoming Sessions</Text>
        {sessionsQuery.isError ? (
          <Text style={styles.error}>Unable to load scheduled sessions.</Text>
        ) : scheduledSessions.length === 0 ? (
          <Text style={styles.meta}>No scheduled sessions in the selected range.</Text>
        ) : (
          <>
            {scheduledSessions.map(sessionItem => {
              const adjustmentReasons = collectAdjustmentReasons(sessionItem.exercises);

              return (
                <View key={`${sessionItem.programSessionId}-${sessionItem.scheduledDate}`}>
                  <View style={styles.scheduleRow}>
                    <Text style={styles.day}>
                      {formatDateLabel(sessionItem.scheduledDate)} ({dayLabel(sessionItem.dayOfWeek)})
                    </Text>
                    <View style={styles.scheduleCopy}>
                      <Text style={styles.workoutName}>{sessionItem.name}</Text>
                      <Text style={styles.meta}>
                        Block {sessionItem.blockWeekIndex} • {sessionItem.exercises.length} exercise
                        {sessionItem.exercises.length === 1 ? '' : 's'} •{' '}
                        {summarizeExercises(sessionItem.exercises.map(item => item.exerciseName))}
                      </Text>
                      <Text
                        style={styles.meta}
                        testID={`upcoming-session-prescription-${sessionItem.programSessionId}-${sessionItem.scheduledDate}`}>
                        {formatPrescriptionSummary(sessionItem.exercises[0])}
                      </Text>
                      {adjustmentReasons.length > 0 ? (
                        <Text
                          style={styles.adjustmentReason}
                          testID={`upcoming-session-adjustment-${sessionItem.programSessionId}-${sessionItem.scheduledDate}`}>
                          Adjustment: {adjustmentReasons.join(' ')}
                        </Text>
                      ) : null}
                    </View>
                  </View>
                </View>
              );
            })}
            <Button
              label={todaysSession ? "Start Today's Session" : 'No Session Today'}
              disabled={!todaysSession}
              onPress={() => navigation.navigate('WorkoutRunner')}
              testID="start-today-session"
            />
          </>
        )}
      </Card>

      {enrollMutation.isError ? (
        <Text style={styles.error}>Unable to enroll right now. Please try again.</Text>
      ) : null}
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
    color: '#475569',
  },
  programsList: {
    gap: 10,
  },
  programName: {
    fontSize: 18,
    fontWeight: '700',
    color: '#0f172a',
  },
  summary: {
    color: '#334155',
  },
  meta: {
    color: '#64748b',
  },
  sectionTitle: {
    fontSize: 18,
    fontWeight: '700',
    color: '#0f172a',
  },
  scheduleRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 10,
  },
  day: {
    minWidth: 90,
    fontWeight: '600',
    color: '#0f172a',
  },
  scheduleCopy: {
    flex: 1,
  },
  workoutName: {
    color: '#0f172a',
    fontWeight: '600',
  },
  error: {
    color: '#b91c1c',
    fontSize: 14,
  },
  adjustmentReason: {
    color: '#0f766e',
    fontSize: 13,
  },
});
