package flag

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"k8s.io/apimachinery/pkg/util/duration"
)

// Wrapper around time.Duration with a parser that accepts days, months, years as valid units.
type Duration struct {
	time.Duration
}

// unit map: symbol -> seconds
var unitMap = map[string]uint64{
	"ns": uint64(time.Nanosecond),
	"us": uint64(time.Microsecond),
	"µs": uint64(time.Microsecond), // U+00B5 = micro symbol
	"μs": uint64(time.Microsecond), // U+03BC = Greek letter mu
	"ms": uint64(time.Millisecond),
	"s":  uint64(time.Second),
	"m":  uint64(time.Minute),
	"h":  uint64(time.Hour),
	"d":  uint64(24 * time.Hour),
	"w":  uint64(7 * 24 * time.Hour),
	"mo": uint64(30 * 24 * time.Hour),
	"y":  uint64(365 * 24 * time.Hour),
}

// ParseDuration parses strings like "2d5h10.5m"
// it does not support negative durations.
func ParseDuration(s string) (Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Duration{Duration: 0}, nil
	}

	var total float64
	i := 0
	n := len(s)

	for i < n {
		// Get number (including decimal point)
		j := i
		hasDot := false
		for j < n && (unicode.IsDigit(rune(s[j])) || (s[j] == '.' && !hasDot)) {
			if s[j] == '.' {
				hasDot = true
			}
			j++
		}
		if j == i {
			return Duration{}, fmt.Errorf("expected number at pos %d", i)
		}
		numStr := s[i:j]
		num, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return Duration{}, err
		}

		// Get unit
		k := j
		for k < n && unicode.IsLetter(rune(s[k])) {
			k++
		}
		if k == j {
			return Duration{}, fmt.Errorf("missing unit after number at pos %d", j)
		}
		unit := strings.ToLower(s[j:k])

		// Query value for unit
		val, ok := unitMap[unit]
		if !ok {
			return Duration{}, fmt.Errorf("unknown unit %q", unit)
		}
		// Add to total
		total += num * float64(val)

		i = k
		for i < n && s[i] == ' ' {
			i++
		}
	}

	// Convert to time.Duration (nanoseconds)
	return Duration{Duration: time.Duration(total)}, nil
}

func (d *Duration) String() string { return duration.ShortHumanDuration(d.Duration) }

func (d *Duration) Set(s string) error {
	parsed, err := ParseDuration(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

func (d *Duration) Type() string { return "duration" }
