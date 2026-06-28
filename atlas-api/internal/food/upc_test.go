package food

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeUPC(t *testing.T) {
	t.Parallel()

	normalized, err := NormalizeUPC(" 0-12345-67890-5 ")
	require.NoError(t, err)
	require.Equal(t, "012345678905", normalized)
}

func TestNormalizeUPCRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	_, err := NormalizeUPC("abc123")
	require.Error(t, err)
}
