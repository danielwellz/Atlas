package exercise

import (
	"time"

	"github.com/google/uuid"
)

type Media struct {
	ID              uuid.UUID
	ExerciseID      uuid.UUID
	MediaType       string
	URI             string
	ThumbnailURI    *string
	DurationSeconds *int32
	CreatedAt       time.Time
}

type CatalogExercise struct {
	ID                    uuid.UUID
	Slug                  string
	Name                  string
	PrimaryMuscleGroup    string
	PrimaryMuscles        []string
	SecondaryMuscles      []string
	MovementPattern       string
	MovementTaxonomy      []string
	Contraindications     []string
	ContraindicationTags  []string
	Equipment             []string
	EquipmentRequirements []string
	Difficulty            string
	Description           string
	CreatedAt             time.Time
	Media                 []Media
}

type ListFilter struct {
	Query     string
	Equipment string
	Pattern   string
}

type SubstituteFilter struct {
	Equipment   []string
	InjuryFlags []string
	Limit       int32
}

type EquipmentFit string

const (
	EquipmentFitExact         EquipmentFit = "exact"
	EquipmentFitPartial       EquipmentFit = "partial"
	EquipmentFitNotApplicable EquipmentFit = "not_applicable"
)

type SubstituteWhy struct {
	MatchedPattern []string
	MatchedMuscles []string
	EquipmentFit   EquipmentFit
}

type RankedSubstitute struct {
	Exercise CatalogExercise
	Score    int
	Why      SubstituteWhy
}
