import type { components, operations } from '../generated/openapi';
import { atlasApiClient, getApiErrorMessage } from '../client';
import type { OnboardingProfile } from '../../storage/onboardingStorage';

type OnboardingStatusResponse = components['schemas']['OnboardingStatusResponse'];
type OnboardingPlanResponse = components['schemas']['OnboardingPlanResponse'];
type SubmitOnboardingProfileRequest =
  operations['PutOnboardingProfile']['requestBody']['content']['application/json'];
type TrainingProfile = components['schemas']['TrainingProfile'];

type OnboardingRequestContext = {
  accessToken: string;
};

type SaveOnboardingSelectionsInput = OnboardingRequestContext & {
  profile: OnboardingProfile;
};

const DEFAULT_SESSION_DURATION_MINUTES = 45;
const DAY_ORDER = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];

type MockOnboardingRecord = {
  profile: OnboardingProfile;
};

const mockOnboardingByToken = new Map<string, MockOnboardingRecord>();

function sortUnique(values: string[]): string[] {
  return [...new Set(values)].sort((left, right) => left.localeCompare(right));
}

function normalizeScheduleDays(days: string[]): string[] {
  const rank = new Map<string, number>(DAY_ORDER.map((day, index) => [day.toLowerCase(), index]));

  return [...new Set(days)]
    .map(day => day.trim())
    .filter(Boolean)
    .filter(day => rank.has(day.toLowerCase()))
    .sort((left, right) => rank.get(left.toLowerCase())! - rank.get(right.toLowerCase())!);
}

function clampDaysPerWeek(daysPerWeek: number): number {
  if (daysPerWeek < 1) {
    return 1;
  }

  if (daysPerWeek > 6) {
    return 6;
  }

  return daysPerWeek;
}

function mapGoalToApi(goal: string | null): string {
  if (!goal || !goal.trim()) {
    return 'general_fitness';
  }

  const normalized = goal.trim().toLowerCase();

  switch (normalized) {
    case 'build strength':
      return 'build_strength';
    case 'lose fat':
      return 'lose_fat';
    case 'improve endurance':
      return 'improve_endurance';
    case 'general fitness':
      return 'general_fitness';
    default:
      return normalized.replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '');
  }
}

function buildTrainingProfileRequest(profile: OnboardingProfile): SubmitOnboardingProfileRequest {
  const scheduleDays = normalizeScheduleDays(profile.scheduleDays);
  const injuriesLimitations = sortUnique(profile.injuriesLimitations);
  const modalityPreferences = sortUnique(profile.modalityPreferences);

  return {
    primaryGoal: mapGoalToApi(profile.goal),
    secondaryGoal: null,
    daysPerWeek: clampDaysPerWeek(scheduleDays.length),
    sessionDurationMinutes: DEFAULT_SESSION_DURATION_MINUTES,
    equipmentAccessJson: sortUnique(profile.equipment),
    injuriesLimitationsFlags:
      injuriesLimitations.length > 0 ? injuriesLimitations : ['none'],
    modalityPreferences:
      modalityPreferences.length > 0 ? modalityPreferences : ['general_fitness'],
    priorTrainingHistory: profile.priorTrainingHistory ?? undefined,
    readinessSignals: profile.readinessSignals ?? undefined,
    constraintsJson: {
      scheduleDays,
    },
  };
}

function getMockRecord(accessToken: string): MockOnboardingRecord {
  const existing = mockOnboardingByToken.get(accessToken);
  if (existing) {
    return existing;
  }

  const created: MockOnboardingRecord = {
    profile: {
      goal: null,
      equipment: [],
      scheduleDays: [],
      injuriesLimitations: [],
      modalityPreferences: [],
      priorTrainingHistory: null,
      readinessSignals: null,
    },
  };

  mockOnboardingByToken.set(accessToken, created);
  return created;
}

function sessionThemeForGoal(goal: string | null): string {
  const normalized = (goal ?? '').trim().toLowerCase();

  if (normalized.includes('strength')) {
    return 'Strength Builder';
  }
  if (normalized.includes('fat')) {
    return 'Conditioning Builder';
  }
  if (normalized.includes('endurance')) {
    return 'Endurance Builder';
  }

  return 'Fitness Builder';
}

function buildFirstWeekPlan(
  goal: string | null,
  scheduleDays: string[],
): OnboardingStatusResponse['firstWeekPlan'] {
  const days = normalizeScheduleDays(scheduleDays);
  if (days.length === 0) {
    return undefined;
  }

  const theme = sessionThemeForGoal(goal);

  return {
    days: days.map((day, index) => ({
      day,
      sessionName: `${theme} ${index + 1}`,
    })),
  };
}

function buildMockTrainingProfile(profile: OnboardingProfile): TrainingProfile {
  const scheduleDays = normalizeScheduleDays(profile.scheduleDays);

  return {
    primaryGoal: mapGoalToApi(profile.goal),
    secondaryGoal: null,
    daysPerWeek: clampDaysPerWeek(scheduleDays.length),
    sessionDurationMinutes: DEFAULT_SESSION_DURATION_MINUTES,
    equipmentAccess: sortUnique(profile.equipment),
    scheduleDays,
    injuriesLimitationsFlags: sortUnique(profile.injuriesLimitations),
    modalityPreferences: sortUnique(profile.modalityPreferences),
    priorTrainingHistory: profile.priorTrainingHistory ?? undefined,
    readinessSignals: profile.readinessSignals ?? undefined,
  };
}

function buildMockPlanExplanation(trainingProfile: TrainingProfile): string {
  const scheduleSummary =
    trainingProfile.scheduleDays.length > 0
      ? trainingProfile.scheduleDays.join(', ')
      : 'your selected schedule';

  const modalities =
    trainingProfile.modalityPreferences.length > 0
      ? trainingProfile.modalityPreferences.join(', ')
      : 'balanced training';

  const limitations =
    trainingProfile.injuriesLimitationsFlags.length > 0
      ? trainingProfile.injuriesLimitationsFlags.join(', ')
      : 'none';

  return `Week one prioritizes ${modalities} across ${trainingProfile.daysPerWeek} sessions (${scheduleSummary}) while accounting for limitations: ${limitations}.`;
}

function buildMockStatus(profile: OnboardingProfile): OnboardingStatusResponse {
  const hasGoal = Boolean(profile.goal && profile.goal.trim());
  const hasEquipment = profile.equipment.length > 0;
  const hasInjuriesLimitations = profile.injuriesLimitations.length > 0;
  const hasModalityPreferences = profile.modalityPreferences.length > 0;
  const scheduleDays = normalizeScheduleDays(profile.scheduleDays);
  const goalsCompleted =
    hasGoal &&
    hasEquipment &&
    hasInjuriesLimitations &&
    hasModalityPreferences &&
    scheduleDays.length > 0;

  const trainingProfile = goalsCompleted ? buildMockTrainingProfile(profile) : undefined;
  const firstWeekPlan = goalsCompleted ? buildFirstWeekPlan(profile.goal, scheduleDays) : undefined;
  const planExplanation =
    goalsCompleted && trainingProfile ? buildMockPlanExplanation(trainingProfile) : undefined;

  return {
    profileCompleted: false,
    goalsCompleted,
    onboardingCompleted: goalsCompleted,
    firstWeekPlan,
    trainingProfile,
    planExplanation,
  };
}

export async function saveOnboardingSelections(
  input: SaveOnboardingSelectionsInput,
  useMockMode: boolean,
): Promise<void> {
  if (useMockMode) {
    const record = getMockRecord(input.accessToken);
    record.profile = {
      goal: input.profile.goal,
      equipment: [...input.profile.equipment],
      scheduleDays: [...input.profile.scheduleDays],
      injuriesLimitations: [...input.profile.injuriesLimitations],
      modalityPreferences: [...input.profile.modalityPreferences],
      priorTrainingHistory: input.profile.priorTrainingHistory
        ? { ...input.profile.priorTrainingHistory }
        : null,
      readinessSignals: input.profile.readinessSignals ? { ...input.profile.readinessSignals } : null,
    };
    return;
  }

  const scheduleDays = normalizeScheduleDays(input.profile.scheduleDays);
  if (scheduleDays.length === 0) {
    throw new Error('Select at least one training day to finish onboarding.');
  }
  if (input.profile.injuriesLimitations.length === 0) {
    throw new Error('Select at least one injuries/limitations option to continue.');
  }
  if (input.profile.modalityPreferences.length === 0) {
    throw new Error('Select at least one modality preference to continue.');
  }

  const response = await atlasApiClient.PUT('/api/v1/onboarding/profile', {
    body: buildTrainingProfileRequest(input.profile),
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to save onboarding selections.'));
  }
}

export async function fetchOnboardingStatus(
  input: OnboardingRequestContext,
  useMockMode: boolean,
): Promise<OnboardingStatusResponse> {
  if (useMockMode) {
    const record = getMockRecord(input.accessToken);
    return buildMockStatus(record.profile);
  }

  const response = await atlasApiClient.GET('/api/v1/onboarding/status', {
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to load onboarding status.'));
  }

  return response.data;
}

export async function fetchOnboardingPlan(
  input: OnboardingRequestContext,
  useMockMode: boolean,
): Promise<OnboardingPlanResponse> {
  if (useMockMode) {
    const record = getMockRecord(input.accessToken);
    const status = buildMockStatus(record.profile);

    if (!status.onboardingCompleted || !status.trainingProfile || !status.firstWeekPlan) {
      throw new Error('Onboarding plan is not ready yet.');
    }

    return {
      trainingProfile: status.trainingProfile,
      firstWeekPlan: status.firstWeekPlan,
      explanation: status.planExplanation ?? 'Your first-week plan is based on your onboarding data.',
    };
  }

  const response = await atlasApiClient.GET('/api/v1/onboarding/plan', {
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to load onboarding plan.'));
  }

  return response.data;
}

export async function syncOnboardingSelections(
  input: SaveOnboardingSelectionsInput,
  useMockMode: boolean,
): Promise<OnboardingStatusResponse> {
  await saveOnboardingSelections(input, useMockMode);
  const status = await fetchOnboardingStatus({ accessToken: input.accessToken }, useMockMode);

  if (!status.onboardingCompleted) {
    return status;
  }

  try {
    const plan = await fetchOnboardingPlan({ accessToken: input.accessToken }, useMockMode);
    return {
      ...status,
      firstWeekPlan: plan.firstWeekPlan,
      trainingProfile: plan.trainingProfile,
      planExplanation: plan.explanation,
    };
  } catch {
    return status;
  }
}
