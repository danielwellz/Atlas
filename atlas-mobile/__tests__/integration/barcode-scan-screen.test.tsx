import React from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import ReactTestRenderer from 'react-test-renderer';
import { BarcodeScanScreen } from '../../src/screens/nutrition/BarcodeScanScreen';
import { createFoodLog, lookupFoodByUpc, searchFoods } from '../../src/api/services/foodService';
import { check } from 'react-native-permissions';

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

jest.mock('../../src/api/services/foodService', () => ({
  createFoodLog: jest.fn(),
  lookupFoodByUpc: jest.fn(),
  searchFoods: jest.fn(),
}));

let mockIsPro = true;

jest.mock('../../src/state/AuthContext', () => ({
  useAuth: () => ({
    session: {
      user: {
        id: 'user-1',
        email: 'athlete@atlas.local',
        isPro: mockIsPro,
        createdAt: '2026-01-01T00:00:00.000Z',
      },
      tokens: {
        accessToken: 'access-token',
        refreshToken: 'refresh-token',
        tokenType: 'Bearer',
        expiresIn: 900,
      },
    },
    logout: jest.fn(async () => {}),
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
      setTimeout(() => resolve(), 0);
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

describe('BarcodeScanScreen integration', () => {
  const mockedLookupFoodByUpc = lookupFoodByUpc as jest.MockedFunction<typeof lookupFoodByUpc>;
  const mockedCreateFoodLog = createFoodLog as jest.MockedFunction<typeof createFoodLog>;
  const mockedSearchFoods = searchFoods as jest.MockedFunction<typeof searchFoods>;
  const mockedCheck = check as jest.MockedFunction<typeof check>;

  beforeEach(() => {
    jest.clearAllMocks();
    mockNavigate.mockReset();
    mockIsPro = true;
    mockedCheck.mockResolvedValue('granted' as Awaited<ReturnType<typeof check>>);
  });

  it('scans barcode, looks up food, and logs after confirmation', async () => {
    const food = {
      id: 'food-1',
      externalId: '012345678905',
      provider: 'edamam',
      label: 'Protein Bar',
      brand: 'Atlas Nutrition',
      nutrients: {
        calories_kcal: 220,
        protein_g: 20,
        carbs_g: 24,
        fat_g: 7,
      },
      createdAt: '2026-02-27T00:00:00.000Z',
      updatedAt: '2026-02-27T00:00:00.000Z',
    };

    mockedLookupFoodByUpc.mockResolvedValue({
      ...food,
    });
    mockedSearchFoods.mockResolvedValue([food]);

    mockedCreateFoodLog.mockResolvedValue({
      id: 'log-1',
      userId: 'user-1',
      datetime: '2026-02-27T00:00:00.000Z',
      foodId: 'food-1',
      quantity: 1,
      unit: 'serving',
      nutrientsSnapshot: {
        calories_kcal: 220,
        protein_g: 20,
        carbs_g: 24,
        fat_g: 7,
      },
      createdAt: '2026-02-27T00:00:00.000Z',
      food: {
        id: 'food-1',
        externalId: '012345678905',
        provider: 'edamam',
        label: 'Protein Bar',
        brand: 'Atlas Nutrition',
        nutrients: {
          calories_kcal: 220,
          protein_g: 20,
          carbs_g: 24,
          fat_g: 7,
        },
        createdAt: '2026-02-27T00:00:00.000Z',
        updatedAt: '2026-02-27T00:00:00.000Z',
      },
    });

    const queryClient = createQueryClient();

    let renderer: ReactTestRenderer.ReactTestRenderer;
    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <BarcodeScanScreen />
        </QueryClientProvider>,
      );
    });

    await flush();

    const camera = renderer!.root.findByProps({ testID: 'barcode-camera' });
    await ReactTestRenderer.act(async () => {
      camera.props.onReadCode({
        nativeEvent: {
          codeStringValue: '012345678905',
        },
      });
    });

    await flush();

    expect(mockedLookupFoodByUpc).toHaveBeenCalledWith({
      accessToken: 'access-token',
      code: '012345678905',
    });
    expect(renderer!.root.findByProps({ testID: 'barcode-candidates-card' })).toBeDefined();
    expect(mockedSearchFoods).toHaveBeenCalledWith({
      accessToken: 'access-token',
      query: 'Protein Bar',
      limit: 5,
    });

    await ReactTestRenderer.act(async () => {
      renderer!.root.findByProps({ testID: 'barcode-candidate-select-food-1' }).props.onPress();
    });
    await flush();
    expect(renderer!.root.findByProps({ testID: 'barcode-confirm-card' })).toBeDefined();

    await ReactTestRenderer.act(async () => {
      renderer!.root.findByProps({ testID: 'barcode-serving-input' }).props.onChangeText('1.5');
    });

    await ReactTestRenderer.act(async () => {
      renderer!.root.findByProps({ testID: 'barcode-log-button' }).props.onPress();
    });

    await flush();
    await flush();

    expect(mockedCreateFoodLog).toHaveBeenCalledWith({
      accessToken: 'access-token',
      foodId: 'food-1',
      quantity: 1.5,
      unit: 'serving',
    });

    await cleanup(renderer!, queryClient);
  });

  it('shows paywall for non-pro users', async () => {
    mockIsPro = false;

    const queryClient = createQueryClient();

    let renderer: ReactTestRenderer.ReactTestRenderer;
    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <BarcodeScanScreen />
        </QueryClientProvider>,
      );
    });

    await flush();

    expect(renderer!.root.findByProps({ testID: 'barcode-paywall' })).toBeDefined();
    expect(renderer!.root.findAllByProps({ testID: 'barcode-camera' })).toHaveLength(0);
    expect(mockedLookupFoodByUpc).not.toHaveBeenCalled();

    await ReactTestRenderer.act(async () => {
      renderer!.root.findByProps({ testID: 'barcode-upsell-button' }).props.onPress();
    });

    expect(mockNavigate).toHaveBeenCalledWith('Paywall', {
      feature: 'barcode_scan',
    });

    await cleanup(renderer!, queryClient);
  });
});
