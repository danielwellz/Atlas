package progression

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	defaultTargetRPE        = 8.0
	loadIncreaseSmallPct    = 0.025
	loadIncreaseLargePct    = 0.05
	loadDeloadModeratePct   = -0.025
	loadDeloadAggressivePct = -0.05
	fatigueHoldThreshold    = 0.55
)

type LoadRecommendationInput struct {
	LastLoadKg        float64
	LastReps          int32
	LastRPE           *float64
	TargetRepsMin     int32
	TargetRepsMax     int32
	TargetRPE         *float64
	LastWeekAdherence float64
	FatigueScore      float64
	Recovered         bool
	AdjustmentReasons []string
	DeloadFlag        bool
}

type LoadRecommendation struct {
	LoadKg float64
	Why    string
}

func RecommendLoad(input LoadRecommendationInput) LoadRecommendation {
	targetRPE := defaultTargetRPE
	if input.TargetRPE != nil && *input.TargetRPE > 0 {
		targetRPE = *input.TargetRPE
	}
	if input.LastWeekAdherence <= 0 {
		input.LastWeekAdherence = 1
	}

	if input.DeloadFlag {
		load := normalizeLoad(input.LastLoadKg * (1 + loadDeloadModeratePct))
		why := "Catch-up week after low adherence; reducing load and volume conservatively."
		if len(input.AdjustmentReasons) > 0 {
			why = input.AdjustmentReasons[0]
		}
		return LoadRecommendation{
			LoadKg: load,
			Why:    why,
		}
	}

	hitTarget := input.LastReps >= input.TargetRepsMax && input.TargetRepsMax > 0
	rpeWithinTarget := input.LastRPE == nil || *input.LastRPE <= targetRPE
	rpeVeryComfortable := input.LastRPE != nil && *input.LastRPE <= targetRPE-1
	missedTarget := input.TargetRepsMin > 0 && input.LastReps < input.TargetRepsMin
	rpeHigh := input.LastRPE != nil && *input.LastRPE > targetRPE+1

	if input.FatigueScore >= fatigueHoldThreshold && !input.Recovered {
		load := normalizeLoad(input.LastLoadKg)
		return LoadRecommendation{
			LoadKg: load,
			Why:    "Recent fatigue signals are elevated; holding load steady this week.",
		}
	}

	if hitTarget && rpeWithinTarget {
		if input.LastWeekAdherence < 0.67 {
			load := normalizeLoad(input.LastLoadKg)
			return LoadRecommendation{
				LoadKg: load,
				Why:    "Target reps were hit, but weekly adherence was low; holding load steady.",
			}
		}

		pct := loadIncreaseSmallPct
		if rpeVeryComfortable {
			pct = loadIncreaseLargePct
		}

		load := normalizeLoad(input.LastLoadKg * (1 + pct))
		return LoadRecommendation{
			LoadKg: load,
			Why:    fmt.Sprintf("Last week hit target reps at manageable effort; increasing load by %.1f%%.", pct*100),
		}
	}

	if missedTarget && rpeHigh {
		load := normalizeLoad(input.LastLoadKg * (1 + loadDeloadAggressivePct))
		return LoadRecommendation{
			LoadKg: load,
			Why:    "Last week missed reps with high effort; slight deload to rebuild momentum.",
		}
	}

	if missedTarget || rpeHigh {
		load := normalizeLoad(input.LastLoadKg)
		return LoadRecommendation{
			LoadKg: load,
			Why:    "Last week trend suggests high fatigue; holding load steady.",
		}
	}

	load := normalizeLoad(input.LastLoadKg)
	return LoadRecommendation{
		LoadKg: load,
		Why:    "Performance was stable last week; keeping load steady.",
	}
}

type WeeklyAdjustmentInput struct {
	ScheduledSessions               int32
	CompletedSessions               int32
	CompletedSets                   int32
	HighRpeSets                     int32
	PreviousWeekAdherence           float64
	PreviousWeekDensity             float64
	PreviousConsecutiveLowAdherence int32
	RecoveredPerformance            bool
	HasProgramHistory               bool
}

type WeeklyAdjustment struct {
	LastWeekAdherence            float64
	LastWeekScheduledSessions    int32
	LastWeekCompletedSessions    int32
	LastWeekDensity              float64
	LastWeekHighRpeRate          float64
	FatigueScore                 float64
	ConsecutiveLowAdherenceWeeks int32
	DeloadFlag                   bool
	Reasons                      []string
}

func ComputeWeeklyAdjustment(input WeeklyAdjustmentInput) WeeklyAdjustment {
	scheduled := maxInt32(input.ScheduledSessions, 0)
	completed := maxInt32(input.CompletedSessions, 0)
	if scheduled > 0 && completed > scheduled {
		completed = scheduled
	}

	completedSets := maxInt32(input.CompletedSets, 0)
	highRpeSets := maxInt32(input.HighRpeSets, 0)
	if completedSets > 0 && highRpeSets > completedSets {
		highRpeSets = completedSets
	}

	adherence := 1.0
	if scheduled > 0 {
		adherence = float64(completed) / float64(scheduled)
	}
	adherence = clampUnit(adherence)

	density := 0.0
	if completed > 0 {
		density = float64(completedSets) / float64(completed)
	}

	highRpeRate := 0.0
	if completedSets > 0 {
		highRpeRate = float64(highRpeSets) / float64(completedSets)
	}
	highRpeRate = clampUnit(highRpeRate)

	densityDrop := 0.0
	if input.PreviousWeekDensity > 0 {
		densityDrop = clampUnit((input.PreviousWeekDensity - density) / input.PreviousWeekDensity)
	}
	if scheduled > 0 && completed == 0 {
		densityDrop = 1
	}

	fatigueScore := clampUnit((1-adherence)*0.5 + highRpeRate*0.3 + densityDrop*0.2)
	if input.RecoveredPerformance {
		fatigueScore = clampUnit(fatigueScore - 0.12)
	}

	if !input.HasProgramHistory {
		adherence = 1
		density = maxFloat64(density, input.PreviousWeekDensity)
		highRpeRate = 0
		fatigueScore = 0
	}

	consecutiveLowAdherence := maxInt32(input.PreviousConsecutiveLowAdherence, 0)
	if scheduled > 0 && adherence < 0.67 {
		consecutiveLowAdherence++
	} else {
		consecutiveLowAdherence = 0
	}

	reasons := make([]string, 0, 4)
	if scheduled > 0 {
		if completed == 0 {
			reasons = append(reasons, fmt.Sprintf("Missed all %d scheduled sessions last week.", scheduled))
		} else if completed < scheduled {
			reasons = append(
				reasons,
				fmt.Sprintf("Completed %d/%d sessions last week (%.0f%% adherence).", completed, scheduled, adherence*100),
			)
		}
	}
	if completedSets > 0 {
		reasons = append(reasons, fmt.Sprintf("High-effort set rate (RPE 9+) was %.0f%%.", highRpeRate*100))
	}
	if input.RecoveredPerformance {
		reasons = append(reasons, "Most recent workout performance recovered toward target effort.")
	}

	deload := false
	if !input.HasProgramHistory {
		reasons = append(reasons, "Baseline week: collecting trend data before forcing a deload.")
	} else {
		switch {
		case scheduled > 0 && completed == 0:
			deload = true
			reasons = append(reasons, "Starting a catch-up deload week after a missed week.")
		case adherence < 0.5 && densityDrop >= 0.2:
			deload = true
			reasons = append(reasons, "Training density dropped with low adherence; starting a deload week.")
		case fatigueScore >= fatigueHoldThreshold && !input.RecoveredPerformance:
			deload = true
			reasons = append(reasons, "Fatigue remained elevated without recovered performance; starting a deload week.")
		case consecutiveLowAdherence >= 2 && adherence < 0.8:
			deload = true
			reasons = append(reasons, "Two low-adherence weeks detected; starting a deload week.")
		default:
			deload = false
			if adherence < 0.85 {
				reasons = append(reasons, "Partial adherence detected; progression will be conservative this week.")
			} else {
				reasons = append(reasons, "Adherence and recovery are stable; progression can continue.")
			}
		}
	}

	return WeeklyAdjustment{
		LastWeekAdherence:            adherence,
		LastWeekScheduledSessions:    scheduled,
		LastWeekCompletedSessions:    completed,
		LastWeekDensity:              density,
		LastWeekHighRpeRate:          highRpeRate,
		FatigueScore:                 fatigueScore,
		ConsecutiveLowAdherenceWeeks: consecutiveLowAdherence,
		DeloadFlag:                   deload,
		Reasons:                      reasons,
	}
}

func AdjustVolumeForCatchUp(baseSets int32, deloadFlag bool) int32 {
	if !deloadFlag {
		return baseSets
	}
	if baseSets <= 1 {
		return 1
	}

	return baseSets - 1
}

func ParseRepsRange(value string) (int32, int32) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, 0
	}

	if strings.Contains(trimmed, "-") {
		parts := strings.SplitN(trimmed, "-", 2)
		min := parsePositiveInt32(parts[0])
		max := parsePositiveInt32(parts[1])
		if min == 0 && max == 0 {
			return 0, 0
		}
		if min == 0 {
			min = max
		}
		if max == 0 {
			max = min
		}
		if max < min {
			min, max = max, min
		}
		return min, max
	}

	valueOnly := parsePositiveInt32(trimmed)
	return valueOnly, valueOnly
}

func parsePositiveInt32(raw string) int32 {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0
	}
	return int32(value)
}

func normalizeLoad(value float64) float64 {
	if !isFinite(value) || value <= 0 {
		return 0
	}
	return math.Round(value*2) / 2
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func clampUnit(value float64) float64 {
	if !isFinite(value) {
		return 0
	}
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func maxInt32(value int32, minimum int32) int32 {
	if value < minimum {
		return minimum
	}
	return value
}

func maxFloat64(left float64, right float64) float64 {
	if left > right {
		return left
	}
	return right
}
