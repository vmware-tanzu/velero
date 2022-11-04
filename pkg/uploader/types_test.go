package uploader

import "testing"

func TestValidateUploaderType(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			"'restic' is a valid type",
			"restic",
			false,
		},
		{
			"'   kopia  ' is a valid type (space will be trimmed)",
			"   kopia  ",
			false,
		},
		{
			"'anything_else' is invalid",
			"anything_else",
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateUploaderType(tt.input); (err != nil) != tt.wantErr {
				t.Errorf("ValidateUploaderType(), input = '%s' error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
