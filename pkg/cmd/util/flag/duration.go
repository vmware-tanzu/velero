package flag

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/duration"
)

// Wrapper around time.Duration with a parser that accepts days, months, years as valid units.
type Duration struct {
	time.Duration
}

// unit map: symbol -> seconds
var unitMap = map[string]int64{
	"yr": 365 * 24 * 3600,
	"mo": 30 * 24 * 3600,
	"w":  7 * 24 * 3600,
	"d":  24 * 3600,
	"h":  3600,
	"m":  60,
	"s":  1,
}

// ParseDuration parses strings like "2d5h10m"
func ParseDuration(s string) (Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Duration{Duration: 0}, nil
	}

	var total int64
	i := 0
	n := len(s)

	for i < n {
		if s[i] < '0' || s[i] > '9' {
			return Duration{}, fmt.Errorf("expected digit at pos %d", i)
		}
		j := i
		for j < n && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		numStr := s[i:j]
		num, err := strconv.ParseInt(numStr, 10, 64)
		if err != nil {
			return Duration{}, err
		}

		// Get unit
		k := j
		for k < n && ((s[k] >= 'a' && s[k] <= 'z') || (s[k] >= 'A' && s[k] <= 'Z')) {
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
		total += num * val

		i = k
		for i < n && s[i] == ' ' {
			i++
		}
	}

	// Convert integer seconds into time.Duration
	return Duration{Duration: time.Duration(total) * time.Second}, nil
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
