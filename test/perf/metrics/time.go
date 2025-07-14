/*
Copyright the Velero contributors.

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

package metrics

import (
	"fmt"
	"time"
)

const TimeCaseDesc = "Time cost"

type TimeSpan struct {
	Start time.Time
	End   time.Time
}

type TimeMetrics struct {
	Name     string
	TimeInfo map[string]TimeSpan // metric name : start timestamp
}

func (t *TimeMetrics) GetMetrics() map[string]string {
	tmpMetrics := make(map[string]string)
	for k, v := range t.TimeInfo {
		duration := v.End.Sub(v.Start)
		if duration < time.Second {
			// For those too shoter time difference we should ignored
			// as it may not really execute the logic
			continue
		}
		tmpMetrics[k] = duration.String() + fmt.Sprintf(" (%s - %s)", v.Start.Format(time.RFC3339), v.End.Format(time.RFC3339))
	}
	return tmpMetrics
}

func (t *TimeMetrics) Start(name string) {
	t.TimeInfo[name] = TimeSpan{
		Start: time.Now(),
	}
}

func (t *TimeMetrics) End(name string) {
	if _, ok := t.TimeInfo[name]; !ok {
		return
	}
	timeSpan := t.TimeInfo[name]
	timeSpan.End = time.Now()
	t.TimeInfo[name] = timeSpan
}

func (*TimeMetrics) Update() error {
	return nil
}

func (*TimeMetrics) GetMetricsName() string {
	return TimeCaseDesc
}
