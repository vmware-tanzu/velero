package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildUserAgent(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		os       string
		arch     string
		gitSha   string
		version  string
		expected string
	}{
		{
			name:     "Test general interpolation in correct order",
			command:  "ark",
			os:       "darwin",
			arch:     "amd64",
			gitSha:   "abc123",
			version:  "v0.1.1",
			expected: "ark/v0.1.1 (darwin/amd64) abc123",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp := buildUserAgent(test.command, test.version, test.gitSha, test.os, test.arch)
			assert.Equal(t, resp, test.expected)
		})
	}
}
