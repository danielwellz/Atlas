import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  createCrew,
  createCrewInvite,
  getCoachSessionById,
  getCrew,
  joinCrewByInvite,
  listCoachSessions,
  listCrews,
  type CreateCrewInput,
  type CreateCrewInviteInput,
  type JoinCrewByInviteInput,
} from '../../api/services/communityService';
import { useAuth } from '../../state/AuthContext';

function crewsQueryKey(userId: string | undefined) {
  return ['community', 'crews', userId ?? 'anonymous'] as const;
}

function crewDetailQueryKey(userId: string | undefined, crewId: string | null) {
  return ['community', 'crew', userId ?? 'anonymous', crewId ?? 'none'] as const;
}

function coachSessionsQueryKey(userId: string | undefined) {
  return ['community', 'coach-sessions', userId ?? 'anonymous'] as const;
}

function coachSessionQueryKey(userId: string | undefined, sessionId: string | null) {
  return ['community', 'coach-session', userId ?? 'anonymous', sessionId ?? 'none'] as const;
}

export function useCrewsQuery() {
  const { session } = useAuth();
  const accessToken = session?.tokens.accessToken ?? '';

  return useQuery({
    queryKey: crewsQueryKey(session?.user.id),
    queryFn: () =>
      listCrews({
        accessToken,
      }),
    enabled: Boolean(accessToken),
  });
}

export function useCrewDetailQuery(crewId: string | null) {
  const { session } = useAuth();
  const accessToken = session?.tokens.accessToken ?? '';

  return useQuery({
    queryKey: crewDetailQueryKey(session?.user.id, crewId),
    queryFn: () =>
      getCrew({
        accessToken,
        crewId: crewId ?? '',
      }),
    enabled: Boolean(accessToken) && Boolean(crewId),
  });
}

export function useCreateCrewMutation() {
  const { session } = useAuth();
  const accessToken = session?.tokens.accessToken ?? '';
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (input: Omit<CreateCrewInput, 'accessToken'>) =>
      createCrew({
        accessToken,
        ...input,
      }),
    onSuccess: async payload => {
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: crewsQueryKey(session?.user.id),
        }),
        queryClient.invalidateQueries({
          queryKey: crewDetailQueryKey(session?.user.id, payload.crew.id),
        }),
      ]);
    },
  });
}

export function useCreateCrewInviteMutation() {
  const { session } = useAuth();
  const accessToken = session?.tokens.accessToken ?? '';

  return useMutation({
    mutationFn: (input: Omit<CreateCrewInviteInput, 'accessToken'>) =>
      createCrewInvite({
        accessToken,
        ...input,
      }),
  });
}

export function useJoinCrewByInviteMutation() {
  const { session } = useAuth();
  const accessToken = session?.tokens.accessToken ?? '';
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (input: Omit<JoinCrewByInviteInput, 'accessToken'>) =>
      joinCrewByInvite({
        accessToken,
        ...input,
      }),
    onSuccess: async payload => {
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: crewsQueryKey(session?.user.id),
        }),
        queryClient.invalidateQueries({
          queryKey: crewDetailQueryKey(session?.user.id, payload.crew.id),
        }),
        queryClient.invalidateQueries({
          queryKey: coachSessionsQueryKey(session?.user.id),
        }),
      ]);
    },
  });
}

export function useCoachSessionsQuery() {
  const { session } = useAuth();
  const accessToken = session?.tokens.accessToken ?? '';

  return useQuery({
    queryKey: coachSessionsQueryKey(session?.user.id),
    queryFn: () =>
      listCoachSessions({
        accessToken,
      }),
    enabled: Boolean(accessToken),
  });
}

export function useCoachSessionDetailQuery(sessionId: string | null) {
  const { session } = useAuth();
  const accessToken = session?.tokens.accessToken ?? '';

  return useQuery({
    queryKey: coachSessionQueryKey(session?.user.id, sessionId),
    queryFn: () =>
      getCoachSessionById({
        accessToken,
        sessionId: sessionId ?? '',
      }),
    enabled: Boolean(accessToken) && Boolean(sessionId),
  });
}
