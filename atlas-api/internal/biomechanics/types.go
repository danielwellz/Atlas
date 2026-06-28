package biomechanics

import (
	"encoding/json"

	"github.com/google/uuid"
)

type MuscleHighlight struct {
	MuscleGroup     string  `json:"muscleGroup"`
	ActivationLevel float64 `json:"activationLevel"`
	Role            string  `json:"role"`
	ColorHex        string  `json:"colorHex,omitempty"`
}

type JointAngle struct {
	Joint         string  `json:"joint"`
	MinDegrees    float64 `json:"minDegrees"`
	MaxDegrees    float64 `json:"maxDegrees"`
	TargetDegrees float64 `json:"targetDegrees"`
	Unit          string  `json:"unit"`
}

type ExerciseBiomechanics struct {
	ExerciseID        uuid.UUID              `json:"exerciseId"`
	ExerciseSlug      string                 `json:"exerciseSlug"`
	ExerciseName      string                 `json:"exerciseName"`
	AnimationAssetKey string                 `json:"animationAssetKey"`
	AnimationAssetURI string                 `json:"animationAssetUri"`
	RigVersion        string                 `json:"rigVersion"`
	MuscleHighlights  []MuscleHighlight      `json:"muscleHighlights"`
	JointAngles       []JointAngle           `json:"jointAngles"`
	Metadata          map[string]interface{} `json:"metadata"`
	MetadataJSON      json.RawMessage        `json:"-"`
}
