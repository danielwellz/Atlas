package habit

import "time"

type SprintDailyChecklist struct {
	Date             time.Time
	TotalEntries     int32
	CompletedEntries int32
}

type SprintProgress struct {
	TotalDays      int32
	CompletedDays  int32
	CurrentDay     int32
	DaysRemaining  int32
	CompletionPct  float32
	CurrentStreak  int32
	LongestStreak  int32
	CompletedToday bool
}

func CalculateSprintProgress(days []SprintDailyChecklist, startDate, endDate, now time.Time) SprintProgress {
	start := normalizeDate(startDate)
	end := normalizeDate(endDate)
	today := normalizeDate(now)

	totalDays := int32(0)
	if !end.Before(start) {
		totalDays = int32(end.Sub(start).Hours()/24) + 1
	}

	completedDays := int32(0)
	completedToday := false
	streakLogs := make([]DailyLog, 0, len(days))

	for _, day := range days {
		normalizedDate := normalizeDate(day.Date)
		if normalizedDate.Before(start) || normalizedDate.After(end) || normalizedDate.After(today) {
			continue
		}

		completed := day.TotalEntries > 0 && day.CompletedEntries == day.TotalEntries
		if completed {
			completedDays++
		}
		if normalizedDate.Equal(today) {
			completedToday = completed
		}

		streakLogs = append(streakLogs, DailyLog{
			Date:      normalizedDate,
			Completed: completed,
		})
	}

	currentStreak, longestStreak := CalculateStreaks(streakLogs, today)

	currentDay := int32(0)
	switch {
	case today.Before(start):
		currentDay = 0
	case today.After(end):
		currentDay = totalDays
	default:
		currentDay = int32(today.Sub(start).Hours()/24) + 1
	}

	daysRemaining := totalDays - completedDays
	if daysRemaining < 0 {
		daysRemaining = 0
	}

	completionPct := float32(0)
	if totalDays > 0 {
		completionPct = float32(completedDays) * 100 / float32(totalDays)
	}

	return SprintProgress{
		TotalDays:      totalDays,
		CompletedDays:  completedDays,
		CurrentDay:     currentDay,
		DaysRemaining:  daysRemaining,
		CompletionPct:  completionPct,
		CurrentStreak:  currentStreak,
		LongestStreak:  longestStreak,
		CompletedToday: completedToday,
	}
}
