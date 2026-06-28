import AsyncStorage from '@react-native-async-storage/async-storage';

const ONBOARDING_KEY = 'atlas.mobile.onboarding';

export type OnboardingSignalMap = Record<string, string | number | boolean>;

export type OnboardingProfile = {
  goal: string | null;
  equipment: string[];
  scheduleDays: string[];
  injuriesLimitations: string[];
  modalityPreferences: string[];
  priorTrainingHistory: OnboardingSignalMap | null;
  readinessSignals: OnboardingSignalMap | null;
};

export type FirstWeekPlanDay = {
  day: string;
  sessionName: string;
};

export type FirstWeekPlan = {
  days: FirstWeekPlanDay[];
};

export type OnboardingState = {
  profile: OnboardingProfile;
  completed: boolean;
  firstWeekPlan: FirstWeekPlan | null;
  planExplanation: string | null;
};

export const defaultOnboardingProfile: OnboardingProfile = {
  goal: null,
  equipment: [],
  scheduleDays: [],
  injuriesLimitations: [],
  modalityPreferences: [],
  priorTrainingHistory: null,
  readinessSignals: null,
};

function parseSignalMap(value: unknown): OnboardingSignalMap | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return null;
  }

  const parsed: OnboardingSignalMap = {};
  for (const [key, item] of Object.entries(value as Record<string, unknown>)) {
    if (typeof item === 'string' || typeof item === 'number' || typeof item === 'boolean') {
      parsed[key] = item;
    }
  }

  if (Object.keys(parsed).length === 0) {
    return null;
  }

  return parsed;
}

export async function loadOnboardingState(): Promise<OnboardingState> {
  const raw = await AsyncStorage.getItem(ONBOARDING_KEY);

  if (!raw) {
    return {
      profile: defaultOnboardingProfile,
      completed: false,
      firstWeekPlan: null,
      planExplanation: null,
    };
  }

  try {
    const parsed = JSON.parse(raw) as OnboardingState;
    return {
      profile: {
        goal: parsed.profile?.goal ?? null,
        equipment: Array.isArray(parsed.profile?.equipment)
          ? parsed.profile.equipment
          : [],
        scheduleDays: Array.isArray(parsed.profile?.scheduleDays)
          ? parsed.profile.scheduleDays
          : [],
        injuriesLimitations: Array.isArray(parsed.profile?.injuriesLimitations)
          ? parsed.profile.injuriesLimitations
          : [],
        modalityPreferences: Array.isArray(parsed.profile?.modalityPreferences)
          ? parsed.profile.modalityPreferences
          : [],
        priorTrainingHistory: parseSignalMap(parsed.profile?.priorTrainingHistory),
        readinessSignals: parseSignalMap(parsed.profile?.readinessSignals),
      },
      completed: Boolean(parsed.completed),
      firstWeekPlan:
        parsed.firstWeekPlan && Array.isArray(parsed.firstWeekPlan.days)
          ? {
              days: parsed.firstWeekPlan.days
                .filter(day => Boolean(day?.day) && Boolean(day?.sessionName))
                .map(day => ({
                  day: String(day.day),
                  sessionName: String(day.sessionName),
                })),
            }
          : null,
      planExplanation:
        typeof parsed.planExplanation === 'string' && parsed.planExplanation.trim()
          ? parsed.planExplanation
          : null,
    };
  } catch {
    return {
      profile: defaultOnboardingProfile,
      completed: false,
      firstWeekPlan: null,
      planExplanation: null,
    };
  }
}

export async function saveOnboardingState(state: OnboardingState): Promise<void> {
  await AsyncStorage.setItem(ONBOARDING_KEY, JSON.stringify(state));
}
