package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactDSN(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		dsn  string
	}{
		{
			name: "url user password",
			dsn:  "postgres://atlas:secret@localhost:5432/atlas?sslmode=disable",
		},
		{
			name: "url query password",
			dsn:  "postgres://atlas@localhost:5432/atlas?password=secret&sslmode=disable",
		},
		{
			name: "key value style",
			dsn:  "host=localhost user=atlas password=secret dbname=atlas sslmode=disable",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			redacted := redactDSN(testCase.dsn)
			require.NotContains(t, redacted, "secret")
			require.Contains(t, strings.ToLower(redacted), "xxxxx")
		})
	}
}
