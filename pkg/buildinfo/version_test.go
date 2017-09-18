package buildinfo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormattedGitSHA(t *testing.T) {
	tests := []struct {
		name     string
		sha      string
		state    string
		expected string
	}{
		{
			"Clean git state has no suffix",
			"abc123",
			"clean",
			"abc123",
		},
		{
			"Dirty git status includes suffix",
			"abc123",
			"dirty",
			"abc123-dirty",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			GitSHA = test.sha
			GitTreeState = test.state
			assert.Equal(t, FormattedGitSHA(), test.expected)
		})
	}

}
