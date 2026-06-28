package food

import (
	"context"
	"encoding/json"
	"fmt"
)

const (
	USDAProviderName   = "usda"
	EdamamProviderName = "edamam"
)

type Nutrients struct {
	CaloriesKcal *float64 `json:"calories_kcal"`
	ProteinG     *float64 `json:"protein_g"`
	CarbsG       *float64 `json:"carbs_g"`
	FatG         *float64 `json:"fat_g"`
}

func (n Nutrients) Scaled(multiplier float64) Nutrients {
	return Nutrients{
		CaloriesKcal: scaledPointer(n.CaloriesKcal, multiplier),
		ProteinG:     scaledPointer(n.ProteinG, multiplier),
		CarbsG:       scaledPointer(n.CarbsG, multiplier),
		FatG:         scaledPointer(n.FatG, multiplier),
	}
}

func (n Nutrients) MarshalJSON() ([]byte, error) {
	type alias Nutrients
	return json.Marshal(alias(n))
}

func ParseNutrientsJSON(raw json.RawMessage) (Nutrients, error) {
	if len(raw) == 0 {
		return Nutrients{}, nil
	}

	var nutrients Nutrients
	if err := json.Unmarshal(raw, &nutrients); err != nil {
		return Nutrients{}, fmt.Errorf("unmarshal nutrients json: %w", err)
	}
	return nutrients, nil
}

type SearchResult struct {
	ExternalID string
	Label      string
	Brand      string
	Nutrients  Nutrients
}

type Detail struct {
	ExternalID string
	Label      string
	Brand      string
	Nutrients  Nutrients
}

type Provider interface {
	Name() string
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)
	GetDetails(ctx context.Context, externalID string) (Detail, error)
}

type UPCProvider interface {
	Name() string
	LookupUPC(ctx context.Context, upc string) (Detail, error)
}

func scaledPointer(value *float64, multiplier float64) *float64 {
	if value == nil {
		return nil
	}
	scaled := *value * multiplier
	return &scaled
}
