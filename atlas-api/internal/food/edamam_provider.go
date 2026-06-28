package food

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultEdamamHTTPTimeout = 10 * time.Second

type EdamamUPCProvider struct {
	baseURL string
	appID   string
	appKey  string
	client  *http.Client
}

type edamamParserResponse struct {
	Parsed []edamamParserItem `json:"parsed"`
	Hints  []edamamParserItem `json:"hints"`
}

type edamamParserItem struct {
	Food edamamFoodRecord `json:"food"`
}

type edamamFoodRecord struct {
	FoodID    string          `json:"foodId"`
	Label     string          `json:"label"`
	KnownAs   string          `json:"knownAs"`
	Brand     string          `json:"brand"`
	Nutrients edamamNutrients `json:"nutrients"`
}

type edamamNutrients struct {
	CaloriesKcal *float64 `json:"ENERC_KCAL"`
	ProteinG     *float64 `json:"PROCNT"`
	CarbsG       *float64 `json:"CHOCDF"`
	FatG         *float64 `json:"FAT"`
}

func NewEdamamUPCProvider(baseURL string, appID string, appKey string) *EdamamUPCProvider {
	normalizedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if normalizedBaseURL == "" {
		normalizedBaseURL = "https://api.edamam.com"
	}

	return &EdamamUPCProvider{
		baseURL: normalizedBaseURL,
		appID:   strings.TrimSpace(appID),
		appKey:  strings.TrimSpace(appKey),
		client:  &http.Client{Timeout: defaultEdamamHTTPTimeout},
	}
}

func (p *EdamamUPCProvider) Name() string {
	return EdamamProviderName
}

func (p *EdamamUPCProvider) LookupUPC(ctx context.Context, upc string) (Detail, error) {
	normalizedUPC, err := NormalizeUPC(upc)
	if err != nil {
		return Detail{}, err
	}

	if p.appID == "" || p.appKey == "" {
		return Detail{}, fmt.Errorf("edamam app id and app key are required")
	}

	endpoint, err := url.Parse(p.baseURL + "/api/food-database/v2/parser")
	if err != nil {
		return Detail{}, err
	}
	query := endpoint.Query()
	query.Set("upc", normalizedUPC)
	query.Set("app_id", p.appID)
	query.Set("app_key", p.appKey)
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return Detail{}, err
	}

	responseBody, err := p.doRequest(req)
	if err != nil {
		return Detail{}, err
	}

	return parseEdamamUPCResponse(normalizedUPC, responseBody)
}

func (p *EdamamUPCProvider) doRequest(req *http.Request) ([]byte, error) {
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		if len(message) > 250 {
			message = message[:250]
		}
		return nil, fmt.Errorf("edamam request failed with status %d: %s", resp.StatusCode, message)
	}

	return body, nil
}

func parseEdamamUPCResponse(upc string, body []byte) (Detail, error) {
	var payload edamamParserResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return Detail{}, fmt.Errorf("unmarshal edamam parser response: %w", err)
	}

	record, ok := firstEdamamRecord(payload)
	if !ok {
		return Detail{}, ErrFoodNotFound
	}

	externalID := strings.TrimSpace(record.FoodID)
	if externalID == "" {
		externalID = upc
	}

	label := strings.TrimSpace(record.Label)
	if label == "" {
		label = strings.TrimSpace(record.KnownAs)
	}
	if label == "" {
		return Detail{}, ErrFoodNotFound
	}

	return Detail{
		ExternalID: externalID,
		Label:      label,
		Brand:      strings.TrimSpace(record.Brand),
		Nutrients: Nutrients{
			CaloriesKcal: cloneFloatPointer(record.Nutrients.CaloriesKcal),
			ProteinG:     cloneFloatPointer(record.Nutrients.ProteinG),
			CarbsG:       cloneFloatPointer(record.Nutrients.CarbsG),
			FatG:         cloneFloatPointer(record.Nutrients.FatG),
		},
	}, nil
}

func firstEdamamRecord(payload edamamParserResponse) (edamamFoodRecord, bool) {
	if len(payload.Parsed) > 0 {
		return payload.Parsed[0].Food, true
	}
	if len(payload.Hints) > 0 {
		return payload.Hints[0].Food, true
	}
	return edamamFoodRecord{}, false
}
