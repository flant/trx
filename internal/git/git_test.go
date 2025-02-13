package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testVer struct {
	current string
	last    string
	initial string
}

func TestCheckNewVersion_happyPath(t *testing.T) {

	tcs := []testVer{
		{
			current: "v0.0.0",
			last:    "",
			initial: "",
		},
		{
			current: "v0.0.1",
			last:    "v0.0.0",
			initial: "",
		},
		{
			current: "v0.0.1",
			last:    "",
			initial: "v0.0.0",
		},
		{
			current: "v0.0.2",
			last:    "v0.0.1",
			initial: "v0.0.0",
		},
		{
			current: "v0.10.0",
			last:    "v0.9.2",
			initial: "v0.1.0",
		},
	}

	for _, tc := range tcs {
		b, err := IsNewerVersion(tc.current, tc.last, tc.initial)
		assert.NoError(t, err)
		assert.True(t, b)
	}
}

func TestCheckNewVersion_unhappyPath(t *testing.T) {

	tcs := []testVer{
		{
			current: "v0.0.0",
			last:    "",
			initial: "v0.0.0",
		},
		{
			current: "v0.0.0",
			last:    "v0.0.0",
			initial: "v0.0.0",
		},
		{
			current: "v0.0.0",
			last:    "v0.0.0",
			initial: "",
		},
		{
			current: "v0.0.0",
			last:    "",
			initial: "v0.1.0",
		},
		{
			current: "v0.1.0",
			last:    "v0.9.2",
			initial: "v0.1.0",
		},
	}

	for _, tc := range tcs {
		b, err := IsNewerVersion(tc.current, tc.last, tc.initial)
		assert.NoError(t, err)
		assert.False(t, b)
	}
}
