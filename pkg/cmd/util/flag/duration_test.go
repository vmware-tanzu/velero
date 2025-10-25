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
			expected: 2 * 31 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "years only",
			input:    "1y",
			expected: 366 * 24 * time.Hour,
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
			input:    "1D 2h 3M 4s",
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
			input:    "10000000s",
			expected: 10000000 * time.Second,
			wantErr:  false,
		},

		// Fractional values
		{
			name:     "basic fraction",
			input:    "5.5s",
			expected: time.Duration(5.5 * float64(time.Second)),
			wantErr:  false,
		},
		{
			name:     "fraction with no decimal part",
			input:    "5.s",
			expected: time.Duration(5 * float64(time.Second)),
			wantErr:  false,
		},
		{
			name:     "fractional with multiple decimals",
			input:    "1.25h30.5m",
			expected: time.Duration(1.25*float64(time.Hour) + 30.5*float64(time.Minute)),
			wantErr:  false,
		},
		{
			name:     "fractional zero",
			input:    "0.0s",
			expected: 0,
			wantErr:  false,
		},
		{
			name:     "mixed integer and fractional",
			input:    "2h1.5m30s",
			expected: 2*time.Hour + time.Duration(1.5*float64(time.Minute)) + 30*time.Second,
			wantErr:  false,
		},

		// Error cases
		{
			name:     "invalid characters",
			input:    "abc",
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
			name:     "number without unit",
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
			name:     "negative number",
			input:    "-5s",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "multiple decimal points",
			input:    "5.5.5s",
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
