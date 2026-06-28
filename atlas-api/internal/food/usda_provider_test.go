package food

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseUSDASearchResponseParsesLabelAndFallbackNutrients(t *testing.T) {
	t.Parallel()

	response := `{
		"foods": [
			{
				"fdcId": 12345,
				"description": "Greek Yogurt",
				"brandName": "Atlas Dairy",
				"labelNutrients": {
					"calories": {"value": 120},
					"protein": {"value": 15},
					"carbohydrates": {"value": 7},
					"fat": {"value": 2}
				}
			},
			{
				"fdcId": 67890,
				"description": "Banana",
				"foodNutrients": [
					{"nutrientId": 1008, "value": 89},
					{"nutrientId": 1003, "value": 1.1},
					{"nutrientId": 1005, "value": 22.8},
					{"nutrientId": 1004, "value": 0.3}
				]
			}
		]
	}`

	results, err := parseUSDASearchResponse([]byte(response))
	require.NoError(t, err)
	require.Len(t, results, 2)

	require.Equal(t, "12345", results[0].ExternalID)
	require.Equal(t, "Greek Yogurt", results[0].Label)
	require.Equal(t, "Atlas Dairy", results[0].Brand)
	require.NotNil(t, results[0].Nutrients.CaloriesKcal)
	require.NotNil(t, results[0].Nutrients.ProteinG)
	require.NotNil(t, results[0].Nutrients.CarbsG)
	require.NotNil(t, results[0].Nutrients.FatG)
	require.InDelta(t, 120.0, *results[0].Nutrients.CaloriesKcal, 0.001)
	require.InDelta(t, 15.0, *results[0].Nutrients.ProteinG, 0.001)
	require.InDelta(t, 7.0, *results[0].Nutrients.CarbsG, 0.001)
	require.InDelta(t, 2.0, *results[0].Nutrients.FatG, 0.001)

	require.Equal(t, "67890", results[1].ExternalID)
	require.Equal(t, "Banana", results[1].Label)
	require.Equal(t, "", results[1].Brand)
	require.NotNil(t, results[1].Nutrients.CaloriesKcal)
	require.NotNil(t, results[1].Nutrients.ProteinG)
	require.NotNil(t, results[1].Nutrients.CarbsG)
	require.NotNil(t, results[1].Nutrients.FatG)
	require.InDelta(t, 89.0, *results[1].Nutrients.CaloriesKcal, 0.001)
	require.InDelta(t, 1.1, *results[1].Nutrients.ProteinG, 0.001)
	require.InDelta(t, 22.8, *results[1].Nutrients.CarbsG, 0.001)
	require.InDelta(t, 0.3, *results[1].Nutrients.FatG, 0.001)
}

func TestParseUSDADetailResponseHandlesNestedNutrientShapeAndMissingNutrients(t *testing.T) {
	t.Parallel()

	response := `{
		"fdcId": 22222,
		"description": "Oatmeal",
		"brandOwner": "Atlas Foods",
		"foodNutrients": [
			{"nutrient": {"id": 1008}, "amount": 68},
			{"nutrient": {"id": 1003}, "amount": 2.4}
		]
	}`

	detail, err := parseUSDADetailResponse([]byte(response))
	require.NoError(t, err)
	require.Equal(t, "22222", detail.ExternalID)
	require.Equal(t, "Oatmeal", detail.Label)
	require.Equal(t, "Atlas Foods", detail.Brand)
	require.NotNil(t, detail.Nutrients.CaloriesKcal)
	require.NotNil(t, detail.Nutrients.ProteinG)
	require.Nil(t, detail.Nutrients.CarbsG)
	require.Nil(t, detail.Nutrients.FatG)
	require.InDelta(t, 68.0, *detail.Nutrients.CaloriesKcal, 0.001)
	require.InDelta(t, 2.4, *detail.Nutrients.ProteinG, 0.001)
}

func TestParseUSDASearchResponseSkipsFoodsWithoutFdcID(t *testing.T) {
	t.Parallel()

	response := `{
		"foods": [
			{
				"fdcId": 0,
				"description": "Invalid"
			},
			{
				"fdcId": 123,
				"description": "Valid"
			}
		]
	}`

	results, err := parseUSDASearchResponse([]byte(response))
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "123", results[0].ExternalID)
	require.Equal(t, "Valid", results[0].Label)
}

func TestUSDAProviderSearchAdapterRequestsExpectedEndpoint(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/foods/search", r.URL.Path)
		require.Equal(t, "api-key", r.URL.Query().Get("api_key"))
		_, _ = w.Write([]byte(`{"foods":[{"fdcId":321,"description":"Greek Yogurt"}]}`))
	}))
	defer server.Close()

	provider := NewUSDAProvider(server.URL, "api-key", 0)
	provider.client = server.Client()

	results, err := provider.Search(context.Background(), "yogurt", 5)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "321", results[0].ExternalID)
	require.Equal(t, "Greek Yogurt", results[0].Label)
}

func TestUSDAProviderSearchAdapterReturnsHTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "provider unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	provider := NewUSDAProvider(server.URL, "api-key", 0)
	provider.client = server.Client()

	_, err := provider.Search(context.Background(), "yogurt", 5)
	require.Error(t, err)
	require.ErrorContains(t, err, "status 503")
}

func TestUSDAProviderGetDetailsUsesInMemoryCache(t *testing.T) {
	t.Parallel()

	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		_, _ = w.Write([]byte(`{"fdcId":777,"description":"Cached Oats"}`))
	}))
	defer server.Close()

	provider := NewUSDAProvider(server.URL, "api-key", time.Minute)
	provider.client = server.Client()

	first, firstErr := provider.GetDetails(context.Background(), "777")
	second, secondErr := provider.GetDetails(context.Background(), "777")

	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	require.Equal(t, first, second)
	require.Equal(t, int32(1), atomic.LoadInt32(&requestCount))
}

func TestUSDAProviderSearchAdapterHonorsContextTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(120 * time.Millisecond)
		_, _ = w.Write([]byte(`{"foods":[]}`))
	}))
	defer server.Close()

	provider := NewUSDAProvider(server.URL, "api-key", 0)
	provider.client = server.Client()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer cancel()

	_, err := provider.Search(ctx, "timeout", 5)
	require.Error(t, err)
	require.ErrorContains(t, err, "context deadline")
}
