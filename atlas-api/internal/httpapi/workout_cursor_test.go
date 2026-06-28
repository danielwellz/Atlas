package httpapi

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestWorkoutHistoryCursorRoundTrip(t *testing.T) {
	t.Parallel()

	expected := workoutHistoryCursor{
		StartedAt: time.Date(2026, time.February, 26, 15, 4, 5, 123456000, time.UTC),
		WorkoutID: uuid.New(),
	}

	encoded := encodeWorkoutHistoryCursor(expected)
	decoded, err := decodeWorkoutHistoryCursor(encoded)
	require.NoError(t, err)
	require.Equal(t, expected.StartedAt, decoded.StartedAt)
	require.Equal(t, expected.WorkoutID, decoded.WorkoutID)
}

func TestDecodeWorkoutHistoryCursorRejectsInvalidPayload(t *testing.T) {
	t.Parallel()

	_, err := decodeWorkoutHistoryCursor("not-base64")
	require.Error(t, err)

	malformed := "MjAyNi0wMi0yNlQxNTowNDowNVp8bm90LWEtdXVpZC1hdC1hbGw"
	_, err = decodeWorkoutHistoryCursor(malformed)
	require.Error(t, err)
}
