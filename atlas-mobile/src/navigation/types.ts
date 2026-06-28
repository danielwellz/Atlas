export type AuthStackParamList = {
  Login: undefined;
  Register: undefined;
};

export type OnboardingStackParamList = {
  Goals: undefined;
  Equipment: undefined;
  Limitations: undefined;
  Preferences: undefined;
  Schedule: undefined;
  Readiness: undefined;
  MomentumSprintEnrollment: {
    readinessProvided: boolean;
  };
};

export type MainTabParamList = {
  Dashboard: undefined;
  Crew: undefined;
  Paywall: {
    feature:
      | 'barcode_scan'
      | 'deep_nutrition'
      | 'biomechanics_overlays'
      | 'form_check_upload'
      | 'coach_tier_pro'
      | 'coach_tier_elite';
  };
  CoachSessionPlayer: {
    sessionId: string;
  };
  Anatomy: undefined;
  Food: undefined;
  MealPlan: undefined;
  BarcodeScan: undefined;
  WeeklyCheckIn: undefined;
  Programs: undefined;
  WorkoutRunner: undefined;
  FormCheck: undefined;
  PrivacySettings: undefined;
};
