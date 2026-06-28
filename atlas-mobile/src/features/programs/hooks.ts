import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  enrollInProgram,
  fetchCurrentProgramSessions,
  fetchCurrentWeekSchedule,
  listPrograms,
} from '../../api/services/programsService';
import { useAuth } from '../../state/AuthContext';
import { useMockMode } from '../../state/MockModeContext';

function resolveProgramsMode(isMockMode: boolean): boolean {
  return __DEV__ && isMockMode;
}

function programsQueryKey(mockModeEnabled: boolean) {
  return ['programs', mockModeEnabled] as const;
}

function currentWeekQueryKey(userId: string | undefined, mockModeEnabled: boolean) {
  return ['programs', 'current-week', userId ?? 'anonymous', mockModeEnabled] as const;
}

function currentSessionsQueryKey(
  userId: string | undefined,
  mockModeEnabled: boolean,
  from: string,
  to: string,
) {
  return ['programs', 'current-sessions', userId ?? 'anonymous', mockModeEnabled, from, to] as const;
}

export function useProgramsQuery() {
  const { isMockMode } = useMockMode();
  const useMockPrograms = resolveProgramsMode(isMockMode);

  return useQuery({
    queryKey: programsQueryKey(useMockPrograms),
    queryFn: () => listPrograms(useMockPrograms),
  });
}

export function useCurrentWeekScheduleQuery() {
  const { session } = useAuth();
  const { isMockMode } = useMockMode();
  const useMockPrograms = resolveProgramsMode(isMockMode);
  const accessToken = session?.tokens.accessToken ?? '';

  return useQuery({
    queryKey: currentWeekQueryKey(session?.user.id, useMockPrograms),
    queryFn: () =>
      fetchCurrentWeekSchedule(
        {
          accessToken,
        },
        useMockPrograms,
      ),
    enabled: useMockPrograms || Boolean(accessToken),
  });
}

export function useCurrentProgramSessionsQuery(from: string, to: string) {
  const { session } = useAuth();
  const { isMockMode } = useMockMode();
  const useMockPrograms = resolveProgramsMode(isMockMode);
  const accessToken = session?.tokens.accessToken ?? '';

  return useQuery({
    queryKey: currentSessionsQueryKey(session?.user.id, useMockPrograms, from, to),
    queryFn: () =>
      fetchCurrentProgramSessions(
        {
          accessToken,
          from,
          to,
        },
        useMockPrograms,
      ),
    enabled: useMockPrograms || Boolean(accessToken),
  });
}

export function useEnrollProgramMutation() {
  const { session } = useAuth();
  const { isMockMode } = useMockMode();
  const useMockPrograms = resolveProgramsMode(isMockMode);
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (programId: string) =>
      enrollInProgram(
        {
          accessToken: session?.tokens.accessToken ?? '',
          programId,
        },
        useMockPrograms,
      ),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: programsQueryKey(useMockPrograms) }),
        queryClient.invalidateQueries({
          queryKey: currentWeekQueryKey(session?.user.id, useMockPrograms),
        }),
        queryClient.invalidateQueries({
          queryKey: ['programs', 'current-sessions', session?.user.id ?? 'anonymous', useMockPrograms],
        }),
        queryClient.invalidateQueries({ queryKey: ['workout', useMockPrograms] }),
      ]);
    },
  });
}
