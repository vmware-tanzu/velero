/*
Copyright 2020 the Velero contributors.

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

package backoff

import (
	"math/rand"
	"time"
)

// BackoffPolicy implements a backoff policy, randomizing its delays
// and saturating at the final value in Millis.
//
// source: https://blog.gopheracademy.com/advent-2014/backoff/
type BackoffPolicy struct {
	Millis []int
}

// Default is a backoff policy ranging up to 5 seconds.
var Default = BackoffPolicy{
	[]int{0, 10, 10, 100, 100, 500, 500, 3000, 3000, 5000},
}

// Duration returns the time duration of the n'th wait cycle in a
// backoff policy. This is b.Millis[n], randomized to avoid thundering
// herds.
func (b BackoffPolicy) Duration(n int) time.Duration {
	if n >= len(b.Millis) {
		n = len(b.Millis) - 1
	}

	return time.Duration(jitter(b.Millis[n])) * time.Millisecond
}

// jitter returns a random integer uniformly distributed in the range
// [0.5 * millis .. 1.5 * millis]
func jitter(millis int) int {
	if millis == 0 {
		return 0
	}

	return millis/2 + rand.Intn(millis)
}
