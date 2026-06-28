package exercise

import (
	"sort"
	"strings"
)

type scoredSubstitute struct {
	RankedSubstitute
	patternMatchesCount int
	primaryMatchCount   int
	secondaryMatchCount int
}

func rankSubstitutes(
	prescribed CatalogExercise,
	candidates []CatalogExercise,
	filter SubstituteFilter,
) []RankedSubstitute {
	limit := int(filter.Limit)
	if limit <= 0 {
		limit = 5
	}

	availableEquipment := normalizeTokens(filter.Equipment)
	injuryFlags := toTokenSet(filter.InjuryFlags)

	prescribedPattern := normalizeMovementTaxonomy(
		prescribed.MovementTaxonomy,
		prescribed.MovementPattern,
	)
	prescribedPrimary := normalizeTokens(prescribed.PrimaryMuscles)
	prescribedSecondary := normalizeTokens(prescribed.SecondaryMuscles)

	scored := make([]scoredSubstitute, 0, len(candidates))
	for _, candidate := range candidates {
		candidateContraTags := candidate.ContraindicationTags
		if len(candidateContraTags) == 0 {
			candidateContraTags = candidate.Contraindications
		}
		if intersectsTokenSet(candidateContraTags, injuryFlags) {
			continue
		}

		candidatePattern := normalizeMovementTaxonomy(
			candidate.MovementTaxonomy,
			candidate.MovementPattern,
		)
		matchedPattern := intersectionOrdered(prescribedPattern, candidatePattern)
		if len(matchedPattern) == 0 {
			continue
		}

		candidatePrimary := normalizeTokens(candidate.PrimaryMuscles)
		candidateSecondary := normalizeTokens(candidate.SecondaryMuscles)

		matchedPrimary := intersectionOrdered(prescribedPrimary, candidatePrimary)
		matchedSecondary := intersectionOrdered(prescribedSecondary, candidateSecondary)
		crossPrimarySecondary := intersectionOrdered(prescribedPrimary, candidateSecondary)
		crossSecondaryPrimary := intersectionOrdered(prescribedSecondary, candidatePrimary)

		if len(matchedPrimary) == 0 && len(matchedSecondary) == 0 && len(crossPrimarySecondary) == 0 {
			continue
		}

		equipmentRequirements := candidate.EquipmentRequirements
		if len(equipmentRequirements) == 0 {
			equipmentRequirements = candidate.Equipment
		}
		equipmentFit, equipmentScore := resolveEquipmentFit(equipmentRequirements, availableEquipment)
		if equipmentFit == "" {
			continue
		}

		matchedMuscles := mergeOrdered(
			matchedPrimary,
			matchedSecondary,
			crossPrimarySecondary,
			crossSecondaryPrimary,
		)

		score := 0
		if strings.EqualFold(strings.TrimSpace(candidate.MovementPattern), strings.TrimSpace(prescribed.MovementPattern)) {
			score += 60
		}
		score += len(matchedPattern) * 18
		score += len(matchedPrimary) * 20
		score += len(matchedSecondary) * 10
		score += len(crossPrimarySecondary) * 6
		score += len(crossSecondaryPrimary) * 4
		score += equipmentScore

		scored = append(scored, scoredSubstitute{
			RankedSubstitute: RankedSubstitute{
				Exercise: candidate,
				Score:    score,
				Why: SubstituteWhy{
					MatchedPattern: matchedPattern,
					MatchedMuscles: matchedMuscles,
					EquipmentFit:   equipmentFit,
				},
			},
			patternMatchesCount: len(matchedPattern),
			primaryMatchCount:   len(matchedPrimary),
			secondaryMatchCount: len(matchedSecondary),
		})
	}

	sort.SliceStable(scored, func(left, right int) bool {
		l := scored[left]
		r := scored[right]
		switch {
		case l.Score != r.Score:
			return l.Score > r.Score
		case l.primaryMatchCount != r.primaryMatchCount:
			return l.primaryMatchCount > r.primaryMatchCount
		case l.secondaryMatchCount != r.secondaryMatchCount:
			return l.secondaryMatchCount > r.secondaryMatchCount
		case l.patternMatchesCount != r.patternMatchesCount:
			return l.patternMatchesCount > r.patternMatchesCount
		case l.Exercise.Name != r.Exercise.Name:
			return l.Exercise.Name < r.Exercise.Name
		default:
			return l.Exercise.ID.String() < r.Exercise.ID.String()
		}
	})

	if len(scored) > limit {
		scored = scored[:limit]
	}

	result := make([]RankedSubstitute, 0, len(scored))
	for _, candidate := range scored {
		result = append(result, candidate.RankedSubstitute)
	}

	return result
}

func resolveEquipmentFit(requirements []string, available []string) (EquipmentFit, int) {
	normalizedRequirements := normalizeTokens(requirements)
	if len(available) == 0 {
		return EquipmentFitNotApplicable, 0
	}

	if len(normalizedRequirements) == 0 {
		return EquipmentFitExact, 10
	}

	availableSet := toTokenSet(available)
	matchCount := 0
	for _, requirement := range normalizedRequirements {
		if _, ok := availableSet[requirement]; ok {
			matchCount++
		}
	}

	if matchCount == 0 {
		return "", 0
	}
	if matchCount == len(normalizedRequirements) {
		return EquipmentFitExact, 10
	}

	return EquipmentFitPartial, 5
}

func normalizeMovementTaxonomy(taxonomy []string, movementPattern string) []string {
	normalized := normalizeTokens(taxonomy)
	pattern := normalizeToken(movementPattern)
	if pattern == "" {
		return normalized
	}
	if !containsToken(normalized, pattern) {
		normalized = append([]string{pattern}, normalized...)
	}
	return normalized
}

func mergeOrdered(values ...[]string) []string {
	merged := make([]string, 0)
	seen := map[string]struct{}{}
	for _, set := range values {
		for _, rawValue := range set {
			token := normalizeToken(rawValue)
			if token == "" {
				continue
			}
			if _, ok := seen[token]; ok {
				continue
			}
			seen[token] = struct{}{}
			merged = append(merged, token)
		}
	}
	return merged
}

func normalizeTokens(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		token := normalizeToken(value)
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		normalized = append(normalized, token)
	}
	return normalized
}

func normalizeToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func containsToken(values []string, target string) bool {
	normalizedTarget := normalizeToken(target)
	if normalizedTarget == "" {
		return false
	}
	for _, value := range values {
		if normalizeToken(value) == normalizedTarget {
			return true
		}
	}
	return false
}

func toTokenSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		token := normalizeToken(value)
		if token == "" {
			continue
		}
		set[token] = struct{}{}
	}
	return set
}

func intersectsTokenSet(values []string, set map[string]struct{}) bool {
	if len(set) == 0 {
		return false
	}
	for _, value := range values {
		if _, ok := set[normalizeToken(value)]; ok {
			return true
		}
	}
	return false
}

func intersectionOrdered(reference []string, candidates []string) []string {
	if len(reference) == 0 || len(candidates) == 0 {
		return []string{}
	}

	candidateSet := toTokenSet(candidates)
	intersection := make([]string, 0)
	seen := map[string]struct{}{}
	for _, token := range reference {
		normalized := normalizeToken(token)
		if normalized == "" {
			continue
		}
		if _, ok := candidateSet[normalized]; !ok {
			continue
		}
		if _, alreadyAdded := seen[normalized]; alreadyAdded {
			continue
		}
		seen[normalized] = struct{}{}
		intersection = append(intersection, normalized)
	}

	return intersection
}
