package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetName(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected string
	}{
		{
			name:     "image name with registry hostname and tag",
			image:    "gcr.io/my-repo/my-image:latest",
			expected: "my-image",
		},
		{
			name:     "image name with registry hostname, without tag",
			image:    "gcr.io/my-repo/my-image",
			expected: "my-image",
		},
		{
			name:     "image name without registry hostname, with tag",
			image:    "my-repo/my-image:latest",
			expected: "my-image",
		},
		{
			name:     "image name without registry hostname, without tag",
			image:    "my-repo/my-image",
			expected: "my-image",
		},
		{
			name:     "image name with registry hostname and port, and tag",
			image:    "mycustomregistry.io:8080/my-repo/my-image:latest",
			expected: "my-image",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, getName(test.image))
		})
	}
}
