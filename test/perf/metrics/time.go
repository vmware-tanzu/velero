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

import "time"

const TimeCaseDesc = "Time cost"

type TimeMetrics struct {
	Name     string
	TimeInfo map[string]time.Time // metric name : start timestamp
	Metrics  map[string]float64   // metric name : time duration
}

func (t *TimeMetrics) GetMetrics() map[string]string {
	tmpMetrics := make(map[string]string)
	for k, v := range t.Metrics {
		duration := time.Duration(v) * time.Second
		tmpMetrics[k] = duration.String()
	}
	return tmpMetrics
}

func (t *TimeMetrics) Start(name string) {
	t.TimeInfo[name] = time.Now()
}

func (t *TimeMetrics) End(name string) {
	t.Metrics[name] = time.Now().Sub(t.TimeInfo[name]).Seconds()
	if t.Metrics[name] < 1 {
		// For those too shoter time difference we should ignored
		// as it may not really execute the logic
		delete(t.Metrics, name)
	}
}

func (t *TimeMetrics) Update() error {
	t.Metrics[t.Name] = time.Now().Sub(t.TimeInfo[t.Name]).Seconds()
	return nil
}

func (t *TimeMetrics) GetMetricsName() string {
	return TimeCaseDesc
}
