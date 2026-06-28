package storage

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestS3StorageNormalizeURI(t *testing.T) {
	t.Parallel()

	s, err := NewS3Storage(S3StorageConfig{
		Endpoint:      "localhost:9000",
		AccessKeyID:   "atlasminio",
		SecretAccess:  "atlasminio",
		DefaultBucket: "atlas-assets",
		UseSSL:        false,
	})
	require.NoError(t, err)

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "s3 uri",
			input:    "s3://atlas-assets/biomechanics/back-squat/clip_v1.fbx",
			expected: "s3://atlas-assets/biomechanics/back-squat/clip_v1.fbx",
		},
		{
			name:     "default bucket key",
			input:    "biomechanics/bench-press/clip_v1.fbx",
			expected: "s3://atlas-assets/biomechanics/bench-press/clip_v1.fbx",
		},
		{
			name:     "http object uri",
			input:    "http://localhost:9000/atlas-assets/biomechanics/barbell-row/clip_v1.fbx",
			expected: "s3://atlas-assets/biomechanics/barbell-row/clip_v1.fbx",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			normalized, err := s.NormalizeURI(testCase.input)
			require.NoError(t, err)
			require.Equal(t, testCase.expected, normalized)
		})
	}
}

func TestS3StorageNormalizeURIRequiresBucketForRawKeys(t *testing.T) {
	t.Parallel()

	s, err := NewS3Storage(S3StorageConfig{
		Endpoint:     "localhost:9000",
		AccessKeyID:  "atlasminio",
		SecretAccess: "atlasminio",
		UseSSL:       false,
	})
	require.NoError(t, err)

	_, err = s.NormalizeURI("biomechanics/back-squat/clip_v1.fbx")
	require.Error(t, err)
}
