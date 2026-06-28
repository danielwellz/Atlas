import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import {
  defaultOnboardingProfile,
  loadOnboardingState,
  saveOnboardingState,
  type FirstWeekPlan,
  type OnboardingProfile,
  type OnboardingSignalMap,
  type OnboardingState,
} from '../storage/onboardingStorage';
import { fetchOnboardingStatus, syncOnboardingSelections } from '../api/services/onboardingService';
import { useAuth } from './AuthContext';
import { useMockMode } from './MockModeContext';

type OnboardingContextValue = {
  profile: OnboardingProfile;
  firstWeekPlan: FirstWeekPlan | null;
  planExplanation: string | null;
  isOnboardingComplete: boolean;
  isHydrated: boolean;
  setGoal: (goal: string) => void;
  toggleEquipment: (equipment: string) => void;
  toggleInjuryLimitation: (flag: string) => void;
  toggleModalityPreference: (preference: string) => void;
  toggleScheduleDay: (day: string) => void;
  setPriorTrainingHistory: (history: OnboardingSignalMap | null) => void;
  setReadinessSignals: (signals: OnboardingSignalMap | null) => void;
  saveSelections: () => Promise<void>;
  completeOnboarding: () => Promise<void>;
  resetOnboarding: () => Promise<void>;
};

type OnboardingProviderProps = {
  children: React.ReactNode;
};

const OnboardingContext = createContext<OnboardingContextValue | undefined>(undefined);
const DAY_ORDER = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];

function sortUnique(values: string[]): string[] {
  return [...new Set(values)].sort((a, b) => a.localeCompare(b));
}

function sortScheduleDays(values: string[]): string[] {
  const rank = new Map<string, number>(DAY_ORDER.map((day, index) => [day, index]));

  return [...new Set(values)].sort((left, right) => {
    const leftRank = rank.get(left) ?? Number.MAX_SAFE_INTEGER;
    const rightRank = rank.get(right) ?? Number.MAX_SAFE_INTEGER;

    if (leftRank === rightRank) {
      return left.localeCompare(right);
    }

    return leftRank - rightRank;
  });
}

function mapApiGoalToLabel(goal: string): string {
  const normalized = goal.trim().toLowerCase();
  switch (normalized) {
    case 'build_strength':
      return 'Build strength';
    case 'lose_fat':
      return 'Lose fat';
    case 'improve_endurance':
      return 'Improve endurance';
    case 'general_fitness':
      return 'General fitness';
    default:
      return goal;
  }
}

function mapTrainingProfileToOnboardingProfile(
  trainingProfile: Awaited<ReturnType<typeof fetchOnboardingStatus>>['trainingProfile'],
): Partial<OnboardingProfile> | null {
  if (!trainingProfile) {
    return null;
  }

  return {
    goal: mapApiGoalToLabel(trainingProfile.primaryGoal),
    equipment: Array.isArray(trainingProfile.equipmentAccess)
      ? sortUnique(trainingProfile.equipmentAccess)
      : [],
    scheduleDays: Array.isArray(trainingProfile.scheduleDays)
      ? sortScheduleDays(trainingProfile.scheduleDays)
      : [],
    injuriesLimitations: Array.isArray(trainingProfile.injuriesLimitationsFlags)
      ? sortUnique(trainingProfile.injuriesLimitationsFlags)
      : [],
    modalityPreferences: Array.isArray(trainingProfile.modalityPreferences)
      ? sortUnique(trainingProfile.modalityPreferences)
      : [],
    priorTrainingHistory:
      trainingProfile.priorTrainingHistory &&
      typeof trainingProfile.priorTrainingHistory === 'object' &&
      !Array.isArray(trainingProfile.priorTrainingHistory)
        ? (trainingProfile.priorTrainingHistory as OnboardingSignalMap)
        : null,
    readinessSignals:
      trainingProfile.readinessSignals &&
      typeof trainingProfile.readinessSignals === 'object' &&
      !Array.isArray(trainingProfile.readinessSignals)
        ? (trainingProfile.readinessSignals as OnboardingSignalMap)
        : null,
  };
}

function mapFirstWeekPlan(firstWeekPlan: {
  days: {
    day: string;
    sessionName: string;
  }[];
} | null | undefined): FirstWeekPlan | null {
  if (!firstWeekPlan) {
    return null;
  }

  return {
    days: firstWeekPlan.days.map(item => ({
      day: item.day,
      sessionName: item.sessionName,
    })),
  };
}

export function OnboardingProvider({ children }: OnboardingProviderProps): React.JSX.Element {
  const { session, isAuthenticated, isHydrated: authHydrated } = useAuth();
  const { isMockMode, isHydrated: mockHydrated } = useMockMode();
  const [state, setState] = useState<OnboardingState>({
    profile: defaultOnboardingProfile,
    completed: false,
    firstWeekPlan: null,
    planExplanation: null,
  });
  const [isLocalHydrated, setIsLocalHydrated] = useState(false);
  const [isServerHydrated, setIsServerHydrated] = useState(false);

  const persist = useCallback(async (nextState: OnboardingState) => {
    await saveOnboardingState(nextState);
  }, []);

  const applyServerStatus = useCallback(
    (status: Awaited<ReturnType<typeof fetchOnboardingStatus>>) => {
      setState(current => {
        const profilePatch = mapTrainingProfileToOnboardingProfile(status.trainingProfile);
        const next: OnboardingState = {
          ...current,
          profile: profilePatch
            ? {
                ...current.profile,
                ...profilePatch,
              }
            : current.profile,
          completed: status.onboardingCompleted,
          firstWeekPlan: mapFirstWeekPlan(status.firstWeekPlan),
          planExplanation: status.planExplanation ?? null,
        };
        persist(next).catch(() => {});
        return next;
      });
    },
    [persist],
  );

  useEffect(() => {
    let cancelled = false;

    const hydrate = async () => {
      try {
        const stored = await loadOnboardingState();
        if (!cancelled) {
          setState(stored);
        }
      } finally {
        if (!cancelled) {
          setIsLocalHydrated(true);
        }
      }
    };

    hydrate().catch(() => {
      if (!cancelled) {
        setIsLocalHydrated(true);
      }
    });

    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (!isLocalHydrated || !authHydrated || !mockHydrated) {
      return;
    }

    if (!isAuthenticated || !session?.tokens.accessToken) {
      setState(current => {
        const next: OnboardingState = {
          ...current,
          completed: false,
          firstWeekPlan: null,
          planExplanation: null,
        };

        persist(next).catch(() => {});
        return next;
      });
      setIsServerHydrated(true);
      return;
    }

    let cancelled = false;
    setIsServerHydrated(false);

    const hydrateFromServer = async () => {
      try {
        const status = await fetchOnboardingStatus(
          {
            accessToken: session.tokens.accessToken,
          },
          isMockMode,
        );

        if (!cancelled) {
          applyServerStatus(status);
        }
      } catch {
        if (!cancelled) {
          setState(current => {
            const next: OnboardingState = {
              ...current,
              completed: false,
              firstWeekPlan: null,
              planExplanation: null,
            };
            persist(next).catch(() => {});
            return next;
          });
        }
      } finally {
        if (!cancelled) {
          setIsServerHydrated(true);
        }
      }
    };

    hydrateFromServer().catch(() => {
      if (!cancelled) {
        setIsServerHydrated(true);
      }
    });

    return () => {
      cancelled = true;
    };
  }, [
    applyServerStatus,
    authHydrated,
    isAuthenticated,
    isLocalHydrated,
    isMockMode,
    mockHydrated,
    persist,
    session?.tokens.accessToken,
  ]);

  const setGoal = useCallback(
    (goal: string) => {
      setState(current => {
        const next: OnboardingState = {
          ...current,
          profile: {
            ...current.profile,
            goal,
          },
        };
        persist(next).catch(() => {});
        return next;
      });
    },
    [persist],
  );

  const toggleEquipment = useCallback(
    (equipment: string) => {
      setState(current => {
        const hasEquipment = current.profile.equipment.includes(equipment);
        const nextEquipment = hasEquipment
          ? current.profile.equipment.filter(item => item !== equipment)
          : [...current.profile.equipment, equipment];

        const next: OnboardingState = {
          ...current,
          profile: {
            ...current.profile,
            equipment: sortUnique(nextEquipment),
          },
        };

        persist(next).catch(() => {});
        return next;
      });
    },
    [persist],
  );

  const toggleInjuryLimitation = useCallback(
    (flag: string) => {
      setState(current => {
        const hasFlag = current.profile.injuriesLimitations.includes(flag);
        const nextFlags = hasFlag
          ? current.profile.injuriesLimitations.filter(item => item !== flag)
          : [...current.profile.injuriesLimitations, flag];

        const next: OnboardingState = {
          ...current,
          profile: {
            ...current.profile,
            injuriesLimitations: sortUnique(nextFlags),
          },
        };

        persist(next).catch(() => {});
        return next;
      });
    },
    [persist],
  );

  const toggleModalityPreference = useCallback(
    (preference: string) => {
      setState(current => {
        const hasPreference = current.profile.modalityPreferences.includes(preference);
        const nextPreferences = hasPreference
          ? current.profile.modalityPreferences.filter(item => item !== preference)
          : [...current.profile.modalityPreferences, preference];

        const next: OnboardingState = {
          ...current,
          profile: {
            ...current.profile,
            modalityPreferences: sortUnique(nextPreferences),
          },
        };

        persist(next).catch(() => {});
        return next;
      });
    },
    [persist],
  );

  const toggleScheduleDay = useCallback(
    (day: string) => {
      setState(current => {
        const hasDay = current.profile.scheduleDays.includes(day);
        const nextDays = hasDay
          ? current.profile.scheduleDays.filter(item => item !== day)
          : [...current.profile.scheduleDays, day];

        const next: OnboardingState = {
          ...current,
          profile: {
            ...current.profile,
            scheduleDays: sortScheduleDays(nextDays),
          },
        };

        persist(next).catch(() => {});
        return next;
      });
    },
    [persist],
  );

  const setPriorTrainingHistory = useCallback(
    (history: OnboardingSignalMap | null) => {
      setState(current => {
        const next: OnboardingState = {
          ...current,
          profile: {
            ...current.profile,
            priorTrainingHistory: history,
          },
        };
        persist(next).catch(() => {});
        return next;
      });
    },
    [persist],
  );

  const setReadinessSignals = useCallback(
    (signals: OnboardingSignalMap | null) => {
      setState(current => {
        const next: OnboardingState = {
          ...current,
          profile: {
            ...current.profile,
            readinessSignals: signals,
          },
        };
        persist(next).catch(() => {});
        return next;
      });
    },
    [persist],
  );

  const saveSelections = useCallback(async () => {
    if (!isAuthenticated || !session?.tokens.accessToken) {
      return;
    }

    const status = await syncOnboardingSelections(
      {
        accessToken: session.tokens.accessToken,
        profile: state.profile,
      },
      isMockMode,
    );

    applyServerStatus(status);
  }, [applyServerStatus, isAuthenticated, isMockMode, session?.tokens.accessToken, state.profile]);

  const completeOnboarding = useCallback(async () => {
    await saveSelections();
  }, [saveSelections]);

  const resetOnboarding = useCallback(async () => {
    const next: OnboardingState = {
      profile: defaultOnboardingProfile,
      completed: false,
      firstWeekPlan: null,
      planExplanation: null,
    };

    setState(next);
    await persist(next);
  }, [persist]);

  const value = useMemo(
    () => ({
      profile: state.profile,
      firstWeekPlan: state.firstWeekPlan,
      planExplanation: state.planExplanation,
      isOnboardingComplete: state.completed,
      isHydrated: isLocalHydrated && isServerHydrated,
      setGoal,
      toggleEquipment,
      toggleInjuryLimitation,
      toggleModalityPreference,
      toggleScheduleDay,
      setPriorTrainingHistory,
      setReadinessSignals,
      saveSelections,
      completeOnboarding,
      resetOnboarding,
    }),
    [
      completeOnboarding,
      isLocalHydrated,
      isServerHydrated,
      resetOnboarding,
      saveSelections,
      setGoal,
      setPriorTrainingHistory,
      setReadinessSignals,
      state.completed,
      state.firstWeekPlan,
      state.planExplanation,
      state.profile,
      toggleEquipment,
      toggleInjuryLimitation,
      toggleModalityPreference,
      toggleScheduleDay,
    ],
  );

  return <OnboardingContext.Provider value={value}>{children}</OnboardingContext.Provider>;
}

export function useOnboarding(): OnboardingContextValue {
  const context = useContext(OnboardingContext);

  if (!context) {
    throw new Error('useOnboarding must be used within OnboardingProvider');
  }

  return context;
}
