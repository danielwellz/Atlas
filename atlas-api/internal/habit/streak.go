package habit

import (
	"sort"
	"time"
)

type DailyLog struct {
	Date      time.Time
	Completed bool
}

func CalculateStreaks(logs []DailyLog, now time.Time) (current int32, longest int32) {
	completedByDate := map[time.Time]bool{}
	allDates := map[time.Time]struct{}{}

	for _, log := range logs {
		day := normalizeDate(log.Date)
		completedByDate[day] = log.Completed
		allDates[day] = struct{}{}
	}

	completedDays := make([]time.Time, 0, len(allDates))
	for day := range allDates {
		if completedByDate[day] {
			completedDays = append(completedDays, day)
		}
	}
	sort.Slice(completedDays, func(i, j int) bool {
		return completedDays[i].Before(completedDays[j])
	})

	longest = longestRun(completedDays)
	current = currentRun(completedByDate, normalizeDate(now))
	return current, longest
}

func longestRun(completedDays []time.Time) int32 {
	if len(completedDays) == 0 {
		return 0
	}

	var longest int32 = 1
	var run int32 = 1
	for i := 1; i < len(completedDays); i++ {
		if isPreviousDay(completedDays[i], completedDays[i-1]) {
			run++
		} else {
			run = 1
		}
		if run > longest {
			longest = run
		}
	}
	return longest
}

func currentRun(completedByDate map[time.Time]bool, today time.Time) int32 {
	if completed, ok := completedByDate[today]; ok && !completed {
		return 0
	}

	anchor := time.Time{}
	if completedByDate[today] {
		anchor = today
	} else {
		yesterday := today.AddDate(0, 0, -1)
		if completedByDate[yesterday] {
			anchor = yesterday
		} else {
			return 0
		}
	}

	var run int32 = 0
	for day := anchor; ; day = day.AddDate(0, 0, -1) {
		if !completedByDate[day] {
			break
		}
		run++
	}
	return run
}

func isPreviousDay(day, previous time.Time) bool {
	return normalizeDate(previous).AddDate(0, 0, 1).Equal(normalizeDate(day))
}

func normalizeDate(value time.Time) time.Time {
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}
