/*
Copyright 2018 the Heptio Ark contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package output

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/duration"
)

// timeOrDefault returns a string representation of t if t is not
// a zero value, or else returns defaultVal.
func timeOrDefault(t time.Time, defaultVal string) string {
	if t.IsZero() {
		return defaultVal
	}
	return t.String()
}

// humanReadableDurationSince returns a string representing the amount of
// time that has passed from `when` to now, or the defaultVal string if
// `when` is the zero value or in the future.
func humanReadableDurationSince(when time.Time, defaultVal string) string {
	if when.IsZero() || when.After(time.Now()) {
		return defaultVal
	}

	return duration.ShortHumanDuration(time.Now().Sub(when))
}

// humanReadableTimeFromNow returns a string representing how far in the
// future `when` is (e.g. "2d"), or how far in the past `when` was (e.g.
// "2d ago"), or "n/a" if `when` is the zero value.
func humanReadableTimeFromNow(when time.Time) string {
	if when.IsZero() {
		return "n/a"
	}

	now := time.Now()
	switch {
	case when == now || when.After(now):
		return duration.ShortHumanDuration(when.Sub(now))
	default:
		return fmt.Sprintf("%s ago", duration.ShortHumanDuration(now.Sub(when)))
	}
}
