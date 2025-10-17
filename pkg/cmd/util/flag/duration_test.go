package flag

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		// Valid cases
		{
			name:     "empty string",
			input:    "",
			expected: 0,
			wantErr:  false,
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: 0,
			wantErr:  false,
		},
		{
			name:     "seconds only",
			input:    "30s",
			expected: 30 * time.Second,
			wantErr:  false,
		},
		{
			name:     "minutes only",
			input:    "5m",
			expected: 5 * time.Minute,
			wantErr:  false,
		},
		{
			name:     "hours only",
			input:    "2h",
			expected: 2 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "days only",
			input:    "3d",
			expected: 3 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "weeks only",
			input:    "1w",
			expected: 7 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "months only",
			input:    "2mo",
			expected: 2 * 30 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "years only",
			input:    "1yr",
			expected: 365 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "combined units",
			input:    "2d5h10m30s",
			expected: 2*24*time.Hour + 5*time.Hour + 10*time.Minute + 30*time.Second,
			wantErr:  false,
		},
		{
			name:     "combined with spaces",
			input:    "1d 12h 30m",
			expected: 24*time.Hour + 12*time.Hour + 30*time.Minute,
			wantErr:  false,
		},
		{
			name:     "mixed case units",
			input:    "1D 2H 3M 4S",
			expected: 24*time.Hour + 2*time.Hour + 3*time.Minute + 4*time.Second,
			wantErr:  false,
		},
		{
			name:     "zero values",
			input:    "0d0h0m0s",
			expected: 0,
			wantErr:  false,
		},
		{
			name:     "large numbers",
			input:    "100d",
			expected: 100 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "all units combined",
			input:    "1yr2mo1w3d4h5m6s",
			expected: 365*24*time.Hour + 2*30*24*time.Hour + 7*24*time.Hour + 3*24*time.Hour + 4*time.Hour + 5*time.Minute + 6*time.Second,
			wantErr:  false,
		},

		// Error cases
		{
			name:     "invalid character at start",
			input:    "abc",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "missing unit",
			input:    "123",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "invalid unit",
			input:    "5x",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "number with invalid character",
			input:    "5a5s",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "empty unit",
			input:    "5",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "negative number",
			input:    "-5s",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "decimal number",
			input:    "5.5s",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "unit without number",
			input:    "s",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "mixed valid and invalid",
			input:    "5s10x",
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseDuration(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseDuration(%q) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseDuration(%q) unexpected error: %v", tt.input, err)
				return
			}

			if result.Duration != tt.expected {
				t.Errorf("ParseDuration(%q) = %v, want %v", tt.input, result.Duration, tt.expected)
			}
		})
	}
}
