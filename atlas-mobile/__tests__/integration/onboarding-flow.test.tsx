import React from 'react';
import ReactTestRenderer from 'react-test-renderer';
import AsyncStorage from '@react-native-async-storage/async-storage';
import type { OnboardingProfile } from '../../src/storage/onboardingStorage';
import { RootNavigator } from '../../src/navigation/RootNavigator';
import { OnboardingProvider } from '../../src/state/OnboardingContext';
import {
  fetchOnboardingStatus,
  syncOnboardingSelections,
} from '../../src/api/services/onboardingService';
import { enrollMomentumSprint } from '../../src/api/services/habitService';

jest.mock('../../src/navigation/MainTabsNavigator', () => {
  const React = require('react');
  const { Text } = require('react-native');

  return {
    MainTabsNavigator: () => <Text testID="main-tabs-screen">Main Tabs</Text>,
  };
});

jest.mock('../../src/api/services/onboardingService', () => ({
  fetchOnboardingStatus: jest.fn(),
  saveOnboardingSelections: jest.fn(),
  syncOnboardingSelections: jest.fn(),
}));

jest.mock('../../src/api/services/habitService', () => ({
  enrollMomentumSprint: jest.fn(),
}));

jest.mock('../../src/state/AuthContext', () => ({
  useAuth: () => ({
    session: {
      user: {
        id: 'user-1',
        email: 'athlete@atlas.local',
        createdAt: '2026-01-01T00:00:00.000Z',
      },
      tokens: {
        accessToken: 'access-token',
        refreshToken: 'refresh-token',
        tokenType: 'Bearer',
        expiresIn: 900,
      },
    },
    isAuthenticated: true,
    isHydrated: true,
  }),
}));

jest.mock('../../src/state/MockModeContext', () => ({
  useMockMode: () => ({
    isMockMode: false,
    isHydrated: true,
  }),
}));

function createStatus(profile: OnboardingProfile) {
  const goalsCompleted =
    Boolean(profile.goal?.trim()) &&
    profile.equipment.length > 0 &&
    profile.scheduleDays.length > 0 &&
    profile.injuriesLimitations.length > 0 &&
    profile.modalityPreferences.length > 0;

  return {
    profileCompleted: false,
    goalsCompleted,
    onboardingCompleted: goalsCompleted,
    planExplanation: goalsCompleted
      ? 'Your plan reflects selected schedule and preferences.'
      : undefined,
    firstWeekPlan: goalsCompleted
      ? {
          days: profile.scheduleDays.map((day, index) => ({
            day,
            sessionName: `Strength Builder ${index + 1}`,
          })),
        }
      : undefined,
  };
}

describe('Onboarding flow integration', () => {
  const mockedFetchOnboardingStatus =
    fetchOnboardingStatus as jest.MockedFunction<typeof fetchOnboardingStatus>;
  const mockedSyncOnboardingSelections =
    syncOnboardingSelections as jest.MockedFunction<
      typeof syncOnboardingSelections
    >;
  const mockedEnrollMomentumSprint =
    enrollMomentumSprint as jest.MockedFunction<typeof enrollMomentumSprint>;

  let serverProfile: OnboardingProfile;

  async function flush(): Promise<void> {
    await ReactTestRenderer.act(async () => {
      await new Promise<void>(resolve => {
        setTimeout(() => resolve(), 0);
      });
    });
  }

  async function getByTestID(
    renderer: ReactTestRenderer.ReactTestRenderer,
    testID: string,
  ): Promise<ReactTestRenderer.ReactTestInstance> {
    for (let attempt = 0; attempt < 20; attempt += 1) {
      const matches = renderer.root.findAllByProps({ testID });
      if (matches.length > 0) {
        return matches[0];
      }
      await flush();
    }

    throw new Error(`Unable to find node with testID=${testID}`);
  }

  async function pressByTestID(
    renderer: ReactTestRenderer.ReactTestRenderer,
    testID: string,
  ): Promise<void> {
    await ReactTestRenderer.act(async () => {
      const target = await getByTestID(renderer, testID);
      target.props.onPress();
    });
  }

  beforeEach(async () => {
    jest.clearAllMocks();
    await AsyncStorage.clear();

    serverProfile = {
      goal: null,
      equipment: [],
      scheduleDays: [],
      injuriesLimitations: [],
      modalityPreferences: [],
      priorTrainingHistory: null,
      readinessSignals: null,
    };

    mockedFetchOnboardingStatus.mockImplementation(async () =>
      createStatus(serverProfile),
    );
    mockedSyncOnboardingSelections.mockImplementation(async input => {
      serverProfile = {
        goal: input.profile.goal,
        equipment: [...input.profile.equipment],
        scheduleDays: [...input.profile.scheduleDays],
        injuriesLimitations: [...input.profile.injuriesLimitations],
        modalityPreferences: [...input.profile.modalityPreferences],
        priorTrainingHistory: input.profile.priorTrainingHistory,
        readinessSignals: input.profile.readinessSignals,
      };

      return createStatus(serverProfile);
    });
    mockedEnrollMomentumSprint.mockResolvedValue({
      enrolled: true,
      enrollment: {
        id: 'sprint-1',
        userId: 'user-1',
        goal: 'build_strength',
        startDate: '2026-02-26',
        endDate: '2026-03-11',
        completedAt: null,
        createdAt: '2026-02-26T10:00:00.000Z',
      },
      progress: {
        totalDays: 14,
        completedDays: 0,
        currentDay: 1,
        daysRemaining: 14,
        completionPercent: 0,
        currentStreak: 0,
        longestStreak: 0,
        completedToday: false,
        nextMilestoneDay: 3,
        nextMilestoneLabel: '3-Day Ignition',
      },
      todayChecklist: [],
      milestones: [],
    });
  });

  it('moves from onboarding to main tabs when sprint enrollment is skipped', async () => {
    let renderer: ReactTestRenderer.ReactTestRenderer;

    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <OnboardingProvider>
          <RootNavigator />
        </OnboardingProvider>,
      );
    });

    await flush();

    expect(
      await getByTestID(renderer!, 'onboarding-goals-screen'),
    ).toBeTruthy();

    await pressByTestID(renderer!, 'goal-option-Build strength');
    await pressByTestID(renderer!, 'goals-next-button');

    await flush();
    expect(
      await getByTestID(renderer!, 'onboarding-equipment-screen'),
    ).toBeTruthy();

    await pressByTestID(renderer!, 'equipment-option-Dumbbells');
    await pressByTestID(renderer!, 'equipment-next-button');

    await flush();
    expect(
      await getByTestID(renderer!, 'onboarding-limitations-screen'),
    ).toBeTruthy();

    await pressByTestID(renderer!, 'limitations-option-none');
    await pressByTestID(renderer!, 'limitations-next-button');

    await flush();
    expect(
      await getByTestID(renderer!, 'onboarding-preferences-screen'),
    ).toBeTruthy();

    await pressByTestID(renderer!, 'preferences-option-barbell_strength');
    await pressByTestID(renderer!, 'preferences-next-button');

    await flush();
    expect(
      await getByTestID(renderer!, 'onboarding-schedule-screen'),
    ).toBeTruthy();

    await pressByTestID(renderer!, 'schedule-option-Monday');
    await pressByTestID(renderer!, 'schedule-next-button');

    await flush();
    expect(
      await getByTestID(renderer!, 'onboarding-readiness-screen'),
    ).toBeTruthy();

    await pressByTestID(renderer!, 'readiness-skip-button');

    await flush();
    expect(
      await getByTestID(renderer!, 'onboarding-momentum-sprint-screen'),
    ).toBeTruthy();

    await pressByTestID(renderer!, 'momentum-sprint-skip-button');

    await flush();

    expect(await getByTestID(renderer!, 'main-tabs-screen')).toBeTruthy();
    expect(mockedFetchOnboardingStatus).toHaveBeenCalled();
    expect(mockedSyncOnboardingSelections).toHaveBeenCalledTimes(1);
    expect(mockedEnrollMomentumSprint).not.toHaveBeenCalled();

    await ReactTestRenderer.act(async () => {
      renderer!.unmount();
    });
  });

  it('enrolls user into momentum sprint when selected before leaving onboarding', async () => {
    let renderer: ReactTestRenderer.ReactTestRenderer;

    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <OnboardingProvider>
          <RootNavigator />
        </OnboardingProvider>,
      );
    });

    await flush();
    expect(
      await getByTestID(renderer!, 'onboarding-goals-screen'),
    ).toBeTruthy();

    await pressByTestID(renderer!, 'goal-option-Build strength');
    await pressByTestID(renderer!, 'goals-next-button');
    await flush();
    expect(
      await getByTestID(renderer!, 'onboarding-equipment-screen'),
    ).toBeTruthy();

    await pressByTestID(renderer!, 'equipment-option-Dumbbells');
    await pressByTestID(renderer!, 'equipment-next-button');
    await flush();
    expect(
      await getByTestID(renderer!, 'onboarding-limitations-screen'),
    ).toBeTruthy();

    await pressByTestID(renderer!, 'limitations-option-none');
    await pressByTestID(renderer!, 'limitations-next-button');
    await flush();
    expect(
      await getByTestID(renderer!, 'onboarding-preferences-screen'),
    ).toBeTruthy();

    await pressByTestID(renderer!, 'preferences-option-barbell_strength');
    await pressByTestID(renderer!, 'preferences-next-button');
    await flush();
    expect(
      await getByTestID(renderer!, 'onboarding-schedule-screen'),
    ).toBeTruthy();

    await pressByTestID(renderer!, 'schedule-option-Monday');
    await pressByTestID(renderer!, 'schedule-next-button');
    await flush();
    expect(
      await getByTestID(renderer!, 'onboarding-readiness-screen'),
    ).toBeTruthy();

    await pressByTestID(renderer!, 'readiness-finish-button');
    await flush();

    expect(
      await getByTestID(renderer!, 'onboarding-momentum-sprint-screen'),
    ).toBeTruthy();

    await pressByTestID(renderer!, 'momentum-sprint-start-button');
    await flush();
    await flush();

    expect(mockedEnrollMomentumSprint).toHaveBeenCalledWith({
      accessToken: 'access-token',
      goal: 'Build strength',
    });
    expect(mockedSyncOnboardingSelections).toHaveBeenCalledTimes(1);

    await ReactTestRenderer.act(async () => {
      renderer!.unmount();
    });
  });
});
