import React from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import ReactTestRenderer from 'react-test-renderer';
import { MealPlanScreen } from '../../src/screens/nutrition/MealPlanScreen';
import {
  generateNutritionMealPlan,
  getLatestNutritionMealPlan,
  upsertNutritionMealPlan,
} from '../../src/api/services/nutritionService';

const mockNavigate = jest.fn();

jest.mock('@react-navigation/native', () => ({
  useNavigation: () => ({
    navigate: mockNavigate,
  }),
}));

jest.mock('../../src/api/services/nutritionService', () => ({
  generateNutritionMealPlan: jest.fn(),
  getLatestNutritionMealPlan: jest.fn(),
  upsertNutritionMealPlan: jest.fn(),
}));

jest.mock('../../src/state/AuthContext', () => ({
  useAuth: () => ({
    session: {
      user: {
        id: 'user-1',
        email: 'athlete@atlas.local',
        isPro: true,
        entitlements: ['deep_nutrition', 'coach_tier_pro'],
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

describe('MealPlanScreen', () => {
  const mockedGetLatestNutritionMealPlan =
    getLatestNutritionMealPlan as jest.MockedFunction<typeof getLatestNutritionMealPlan>;
  const mockedGenerateNutritionMealPlan =
    generateNutritionMealPlan as jest.MockedFunction<typeof generateNutritionMealPlan>;
  const mockedUpsertNutritionMealPlan =
    upsertNutritionMealPlan as jest.MockedFunction<typeof upsertNutritionMealPlan>;

  beforeEach(() => {
    jest.clearAllMocks();
    mockNavigate.mockReset();

    mockedGetLatestNutritionMealPlan.mockResolvedValue({
      id: 'plan-1',
      userId: 'user-1',
      week_start: '2026-02-23',
      targets: {
        calories_target: 2400,
        protein_g_target: 170,
        carbs_g_target: 280,
        fat_g_target: 70,
      },
      items: [
        {
          day_of_week: 1,
          meal_slot: 'breakfast',
          servings: 1.2,
          recipe: {
            id: 'recipe-1',
            slug: 'overnight-oats-protein',
            name: 'Protein Overnight Oats',
            meal_type: 'breakfast',
            description: 'Oats with yogurt and berries.',
            servings: 1,
            calories_kcal: 420,
            protein_g: 30,
            carbs_g: 56,
            fat_g: 12,
            ingredients: [
              {
                name: 'rolled oats',
                quantity: 60,
                unit: 'g',
                category: 'grains',
              },
            ],
          },
        },
      ],
      grocery_items: [
        {
          name: 'rolled oats',
          quantity: 504,
          unit: 'g',
          category: 'grains',
        },
      ],
      createdAt: '2026-02-27T10:00:00.000Z',
      updatedAt: '2026-02-27T10:00:00.000Z',
    });

    mockedGenerateNutritionMealPlan.mockResolvedValue({
      id: 'plan-1',
      userId: 'user-1',
      week_start: '2026-02-23',
      targets: {
        calories_target: 2400,
        protein_g_target: 170,
        carbs_g_target: 280,
        fat_g_target: 70,
      },
      items: [],
      grocery_items: [],
      createdAt: '2026-02-27T10:00:00.000Z',
      updatedAt: '2026-02-27T10:00:00.000Z',
    });

    mockedUpsertNutritionMealPlan.mockResolvedValue({
      id: 'plan-1',
      userId: 'user-1',
      week_start: '2026-02-23',
      targets: {
        calories_target: 2400,
        protein_g_target: 170,
        carbs_g_target: 280,
        fat_g_target: 70,
      },
      items: [],
      grocery_items: [],
      createdAt: '2026-02-27T10:00:00.000Z',
      updatedAt: '2026-02-27T10:00:00.000Z',
    });
  });

  it('renders plan and grocery views from latest meal plan', async () => {
    const queryClient = createQueryClient();
    let renderer: ReactTestRenderer.ReactTestRenderer;

    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <MealPlanScreen />
        </QueryClientProvider>,
      );
    });

    await flush();
    await flush();

    const daySection = renderer!.root.findByProps({
      testID: 'meal-plan-day-1',
    });
    expect(daySection).toBeDefined();

    const groceryToggle = renderer!.root.findByProps({
      testID: 'meal-plan-view-grocery',
    });

    await ReactTestRenderer.act(async () => {
      groceryToggle.props.onPress();
    });

    const groceryItem = renderer!.root.findByProps({
      testID: 'grocery-item-rolled oats',
    });
    expect(groceryItem).toBeDefined();

    await ReactTestRenderer.act(async () => {
      renderer!.unmount();
    });
    queryClient.clear();
  });
});
