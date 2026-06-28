import React from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import ReactTestRenderer from 'react-test-renderer';
import { check } from 'react-native-permissions';
import { FormCheckScreen } from '../../src/screens/workout/FormCheckScreen';
import { fetchConsents, grantConsent } from '../../src/api/services/consentService';
import { uploadFormCheckResult } from '../../src/api/services/formCheckService';
import {
  startFormCheckDetection,
  stopFormCheckDetection,
} from '../../src/native/formCheckPose';

const mockNavigate = jest.fn();

jest.mock('@react-navigation/native', () => ({
  useNavigation: () => ({
    navigate: mockNavigate,
  }),
}));

jest.mock('react-native-camera-kit', () => {
  const React = require('react');
  const { View } = require('react-native');

  return {
    Camera: (props: any) => React.createElement(View, { ...props }),
  };
});

jest.mock('react-native-permissions', () => ({
  PERMISSIONS: {
    IOS: {
      CAMERA: 'ios.permission.CAMERA',
    },
    ANDROID: {
      CAMERA: 'android.permission.CAMERA',
    },
  },
  RESULTS: {
    UNAVAILABLE: 'unavailable',
    BLOCKED: 'blocked',
    DENIED: 'denied',
    GRANTED: 'granted',
    LIMITED: 'limited',
  },
  check: jest.fn(),
  request: jest.fn(),
  openSettings: jest.fn(),
}));

jest.mock('../../src/api/services/consentService', () => ({
  fetchConsents: jest.fn(),
  grantConsent: jest.fn(),
}));

jest.mock('../../src/api/services/formCheckService', () => ({
  uploadFormCheckResult: jest.fn(),
}));

let frameListener: ((frame: unknown) => void) | null = null;

jest.mock('../../src/native/formCheckPose', () => ({
  startFormCheckDetection: jest.fn(async () => {}),
  stopFormCheckDetection: jest.fn(async () => ({
    movementType: 'squat',
    sampleCount: 80,
    repetitionCount: 4,
    rangeOfMotionDegrees: 84.2,
    rangeOfMotionScore: 88,
    kneeTrackingScore: 90,
    symmetryScore: 85,
    overallScore: 88,
    feedback: ['Solid rep quality across depth, tracking, and symmetry.'],
    minLeftKneeDeg: 86.2,
    minRightKneeDeg: 87.0,
    maxLeftKneeDeg: 172.0,
    maxRightKneeDeg: 171.3,
  })),
  subscribeToPoseFrames: jest.fn((callback: (frame: unknown) => void) => {
    frameListener = callback;
    return () => {
      frameListener = null;
    };
  }),
  resetFormCheckPoseRuntime: jest.fn(),
}));

let mockEntitlements: string[] = ['form_check_upload', 'coach_tier_pro'];
let mockIsPro = true;

jest.mock('../../src/state/AuthContext', () => ({
  useAuth: () => ({
    session: {
      user: {
        id: 'user-1',
        email: 'athlete@atlas.local',
        isPro: mockIsPro,
        entitlements: mockEntitlements,
        coachTier: 'pro',
        createdAt: '2026-01-01T00:00:00.000Z',
      },
      tokens: {
        accessToken: 'access-token',
        refreshToken: 'refresh-token',
        tokenType: 'Bearer',
        expiresIn: 900,
      },
    },
  }),
}));

jest.mock('../../src/state/MockModeContext', () => ({
  useMockMode: () => ({
    isMockMode: false,
  }),
}));

function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: Infinity,
      },
    },
  });
}

async function flush(): Promise<void> {
  await ReactTestRenderer.act(async () => {
    await new Promise<void>(resolve => {
      setTimeout(resolve, 0);
    });
  });
}

async function cleanup(
  renderer: ReactTestRenderer.ReactTestRenderer,
  queryClient: QueryClient,
): Promise<void> {
  await ReactTestRenderer.act(async () => {
    renderer.unmount();
  });
  queryClient.clear();
}

describe('FormCheckScreen integration', () => {
  const mockedCheck = check as jest.MockedFunction<typeof check>;
  const mockedFetchConsents = fetchConsents as jest.MockedFunction<typeof fetchConsents>;
  const mockedGrantConsent = grantConsent as jest.MockedFunction<typeof grantConsent>;
  const mockedUploadFormCheckResult = uploadFormCheckResult as jest.MockedFunction<
    typeof uploadFormCheckResult
  >;
  const mockedStartDetection = startFormCheckDetection as jest.MockedFunction<
    typeof startFormCheckDetection
  >;
  const mockedStopDetection = stopFormCheckDetection as jest.MockedFunction<typeof stopFormCheckDetection>;

  beforeEach(() => {
    jest.clearAllMocks();
    mockNavigate.mockReset();
    frameListener = null;
    mockEntitlements = ['form_check_upload', 'coach_tier_pro'];
    mockIsPro = true;

    mockedCheck.mockResolvedValue('granted' as Awaited<ReturnType<typeof check>>);
    mockedFetchConsents.mockResolvedValue([
      {
        id: 'consent-form-check-local',
        consentType: 'form_check_local',
        grantedAt: '2026-01-01T00:00:00.000Z',
        revokedAt: null,
        metadataJson: { source: 'test' },
      },
      {
        id: 'consent-form-check-upload',
        consentType: 'form_check_upload',
        grantedAt: '2026-01-01T00:00:00.000Z',
        revokedAt: null,
        metadataJson: { source: 'test' },
      },
    ] as Awaited<ReturnType<typeof fetchConsents>>);

    mockedGrantConsent.mockResolvedValue({
      id: 'consent-updated',
      consentType: 'form_check_local',
      grantedAt: '2026-01-01T00:00:00.000Z',
      revokedAt: null,
      metadataJson: { source: 'test' },
    } as Awaited<ReturnType<typeof grantConsent>>);

    mockedUploadFormCheckResult.mockResolvedValue({
      id: 'upload-1',
      userId: 'user-1',
      movementType: 'squat',
      recordingStartedAt: '2026-02-28T00:00:00.000Z',
      recordingEndedAt: '2026-02-28T00:00:10.000Z',
      summary: {
        overallScore: 88,
        rangeOfMotionScore: 88,
        kneeTrackingScore: 90,
        symmetryScore: 85,
        rangeOfMotionDegrees: 84.2,
        repetitionCount: 4,
        feedback: ['Solid rep quality across depth, tracking, and symmetry.'],
      },
      storageKey: 'form-check-uploads/user-1/upload-1.json',
      metadataJson: {},
      createdAt: '2026-02-28T00:00:12.000Z',
    } as Awaited<ReturnType<typeof uploadFormCheckResult>>);
  });

  it('allows enabling local consent from the screen', async () => {
    mockedFetchConsents.mockResolvedValue([] as Awaited<ReturnType<typeof fetchConsents>>);

    const queryClient = createQueryClient();

    let renderer: ReactTestRenderer.ReactTestRenderer;
    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <FormCheckScreen />
        </QueryClientProvider>,
      );
    });

    await flush();

    expect(renderer!.root.findByProps({ testID: 'form-check-consent-card' })).toBeDefined();

    await ReactTestRenderer.act(async () => {
      renderer!.root.findByProps({ testID: 'form-check-enable-local-consent' }).props.onPress();
    });

    expect(mockedGrantConsent).toHaveBeenCalledWith(
      {
        accessToken: 'access-token',
        consentType: 'form_check_local',
        metadataJson: {
          source: 'form_check_screen',
        },
      },
      false,
    );

    await cleanup(renderer!, queryClient);
  });

  it('runs local analysis and only uploads on explicit tap', async () => {
    const queryClient = createQueryClient();

    let renderer: ReactTestRenderer.ReactTestRenderer;
    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <FormCheckScreen />
        </QueryClientProvider>,
      );
    });

    await flush();

    await ReactTestRenderer.act(async () => {
      renderer!.root.findByProps({ testID: 'form-check-start-button' }).props.onPress();
    });

    expect(mockedStartDetection).toHaveBeenCalledWith('squat');

    await ReactTestRenderer.act(async () => {
      frameListener?.({
        timestampMs: Date.now(),
        leftKneeDeg: 120,
        rightKneeDeg: 118,
        leftHipDeg: 130,
        rightHipDeg: 129,
      });
      frameListener?.({
        timestampMs: Date.now() + 120,
        leftKneeDeg: 100,
        rightKneeDeg: 98,
        leftHipDeg: 112,
        rightHipDeg: 111,
      });
    });

    await ReactTestRenderer.act(async () => {
      renderer!.root.findByProps({ testID: 'form-check-stop-button' }).props.onPress();
    });

    await flush();

    expect(mockedStopDetection).toHaveBeenCalledWith('squat');
    expect(renderer!.root.findByProps({ testID: 'form-check-results-card' })).toBeDefined();
    expect(mockedUploadFormCheckResult).not.toHaveBeenCalled();

    await ReactTestRenderer.act(async () => {
      renderer!.root.findByProps({ testID: 'form-check-upload-button' }).props.onPress();
    });

    await flush();

    expect(mockedUploadFormCheckResult).toHaveBeenCalledTimes(1);

    await cleanup(renderer!, queryClient);
  });

  it('shows upgrade CTA for non-entitled users', async () => {
    mockEntitlements = ['coach_tier_pro'];
    mockIsPro = false;

    const queryClient = createQueryClient();

    let renderer: ReactTestRenderer.ReactTestRenderer;
    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <FormCheckScreen />
        </QueryClientProvider>,
      );
    });

    await flush();

    await ReactTestRenderer.act(async () => {
      renderer!.root.findByProps({ testID: 'form-check-start-button' }).props.onPress();
    });

    await ReactTestRenderer.act(async () => {
      renderer!.root.findByProps({ testID: 'form-check-stop-button' }).props.onPress();
    });

    await flush();

    expect(renderer!.root.findByProps({ testID: 'form-check-upload-paywall' })).toBeDefined();

    await ReactTestRenderer.act(async () => {
      renderer!.root
        .findByProps({ testID: 'form-check-upload-upgrade-button' })
        .props.onPress();
    });

    expect(mockNavigate).toHaveBeenCalledWith('Paywall', {
      feature: 'form_check_upload',
    });

    await cleanup(renderer!, queryClient);
  });
});
