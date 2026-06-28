package food

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseEdamamUPCResponsePrefersParsedResult(t *testing.T) {
	t.Parallel()

	response := `{
		"parsed": [
			{
				"food": {
					"foodId": "food_edamam_1",
					"label": "Protein Bar",
					"brand": "Atlas Nutrition",
					"nutrients": {
						"ENERC_KCAL": 220,
						"PROCNT": 20,
						"CHOCDF": 24,
						"FAT": 7
					}
				}
			}
		]
	}`

	detail, err := parseEdamamUPCResponse("012345678905", []byte(response))
	require.NoError(t, err)
	require.Equal(t, "food_edamam_1", detail.ExternalID)
	require.Equal(t, "Protein Bar", detail.Label)
	require.Equal(t, "Atlas Nutrition", detail.Brand)
	require.NotNil(t, detail.Nutrients.CaloriesKcal)
	require.NotNil(t, detail.Nutrients.ProteinG)
	require.NotNil(t, detail.Nutrients.CarbsG)
	require.NotNil(t, detail.Nutrients.FatG)
	require.InDelta(t, 220.0, *detail.Nutrients.CaloriesKcal, 0.001)
	require.InDelta(t, 20.0, *detail.Nutrients.ProteinG, 0.001)
	require.InDelta(t, 24.0, *detail.Nutrients.CarbsG, 0.001)
	require.InDelta(t, 7.0, *detail.Nutrients.FatG, 0.001)
}

func TestParseEdamamUPCResponseFallsBackToHintsAndUPCExternalID(t *testing.T) {
	t.Parallel()

	response := `{
		"hints": [
			{
				"food": {
					"knownAs": "Greek Yogurt",
					"nutrients": {
						"ENERC_KCAL": 120,
						"PROCNT": 15
					}
				}
			}
		]
	}`

	detail, err := parseEdamamUPCResponse("012345678905", []byte(response))
	require.NoError(t, err)
	require.Equal(t, "012345678905", detail.ExternalID)
	require.Equal(t, "Greek Yogurt", detail.Label)
	require.Equal(t, "", detail.Brand)
	require.NotNil(t, detail.Nutrients.CaloriesKcal)
	require.NotNil(t, detail.Nutrients.ProteinG)
	require.Nil(t, detail.Nutrients.CarbsG)
	require.Nil(t, detail.Nutrients.FatG)
}

func TestParseEdamamUPCResponseReturnsNotFound(t *testing.T) {
	t.Parallel()

	detail, err := parseEdamamUPCResponse("012345678905", []byte(`{"parsed":[],"hints":[]}`))
	require.ErrorIs(t, err, ErrFoodNotFound)
	require.Equal(t, Detail{}, detail)
}

func TestEdamamUPCProviderLookupAdapterRequestsExpectedEndpoint(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/api/food-database/v2/parser", r.URL.Path)
		require.Equal(t, "012345678905", r.URL.Query().Get("upc"))
		require.Equal(t, "app-id", r.URL.Query().Get("app_id"))
		require.Equal(t, "app-key", r.URL.Query().Get("app_key"))
		_, _ = w.Write([]byte(`{"parsed":[{"food":{"foodId":"food_1","label":"Protein Bar"}}]}`))
	}))
	defer server.Close()

	provider := NewEdamamUPCProvider(server.URL, "app-id", "app-key")
	provider.client = server.Client()

	detail, err := provider.LookupUPC(context.Background(), "012345678905")
	require.NoError(t, err)
	require.Equal(t, "food_1", detail.ExternalID)
	require.Equal(t, "Protein Bar", detail.Label)
}

func TestEdamamUPCProviderLookupAdapterReturnsHTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer server.Close()

	provider := NewEdamamUPCProvider(server.URL, "app-id", "app-key")
	provider.client = server.Client()

	_, err := provider.LookupUPC(context.Background(), "012345678905")
	require.Error(t, err)
	require.ErrorContains(t, err, "status 403")
}

func TestEdamamUPCProviderLookupAdapterHonorsContextTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(120 * time.Millisecond)
		_, _ = w.Write([]byte(`{"parsed":[],"hints":[]}`))
	}))
	defer server.Close()

	provider := NewEdamamUPCProvider(server.URL, "app-id", "app-key")
	provider.client = server.Client()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer cancel()

	_, err := provider.LookupUPC(ctx, "012345678905")
	require.Error(t, err)
	require.ErrorContains(t, err, "context deadline")
}
