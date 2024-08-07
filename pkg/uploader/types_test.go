package uploader

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateUploaderType(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
		wantMsg string
	}{
		{
			"'restic' is a valid type",
			"restic",
			"",
			"Uploader 'restic' is deprecated, don't use it for new backups, otherwise the backups won't be available for restore when this functionality is removed in a future version of Velero",
		},
		{
			"'   kopia  ' is a valid type (space will be trimmed)",
			"   kopia  ",
			"",
			"",
		},
		{
			"'anything_else' is invalid",
			"anything_else",
			"invalid uploader type 'anything_else', valid upload types are: 'restic', 'kopia'",
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ValidateUploaderType(tt.input)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.wantMsg, msg)
		})
	}
}
