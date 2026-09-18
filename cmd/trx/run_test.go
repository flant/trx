package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The operator config wins over the repository one, as documented, and neither
// input map is modified: cfg.Env used to be written into in place.
func TestMergeEnvs(t *testing.T) {
	repoEnv := map[string]string{"werf_env": "staging", "only_repo": "1"}
	operatorEnv := map[string]string{"werf_env": "production"}

	merged := mergeEnvs(repoEnv, operatorEnv)

	require.Equal(t, map[string]string{
		"werf_env":  "production",
		"only_repo": "1",
	}, merged)
	require.Equal(t, map[string]string{"werf_env": "staging", "only_repo": "1"}, repoEnv)
	require.Equal(t, map[string]string{"werf_env": "production"}, operatorEnv)
}
