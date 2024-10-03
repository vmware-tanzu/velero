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
	"strings"
	"sync"
	"time"
)

// Metric is an interface for metrics that test needs.
type Metric interface {
	Update() error
	GetMetricsName() string
	GetMetrics() map[string]string
}

// MetricsCollector is a singleton struct for collecting metrics.
type MetricsCollector struct {
	Metrics        []Metric   // metrics need update periodically
	OneTimeMetrics []Metric   // OneTimeMetrics just update one time
	Mu             sync.Mutex // mutex for protecting shared resources
}

// GetMetricsCollector returns the singleton instance of MetricsCollector
func GetMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		Metrics:        []Metric{},
		OneTimeMetrics: []Metric{},
	}
}

// RegisterMetric adds a metric to the MetricsCollector
func (m *MetricsCollector) RegisterMetric(metric Metric) {
	m.Metrics = append(m.Metrics, metric)
}

// RegisterOneTimeMetric adds a one-time metric to the MetricsCollector
func (m *MetricsCollector) RegisterOneTimeMetric(metric Metric) {
	m.OneTimeMetrics = append(m.OneTimeMetrics, metric)
}

// UpdateMetrics periodically updates the metrics for all metrics
func (m *MetricsCollector) UpdateMetrics() {
	for {
		m.Mu.Lock() // Acquire the lock before accessing shared resources
		for _, metric := range m.Metrics {
			if err := metric.Update(); err != nil {
				fmt.Printf("Failed to update metrics: %v\n", err)
				continue
			}
		}
		m.Mu.Unlock()                // Release the lock after accessing shared resources
		time.Sleep(15 * time.Second) // Adjust the interval as per your requirement.
	}
}

// UpdateOneTimeMetrics periodically updates the one-time metrics for all metrics
func (m *MetricsCollector) UpdateOneTimeMetrics() {
	// NotifyOneTimeMonitors notifies the one-time metrics about the metric
	for _, metric := range m.OneTimeMetrics {
		if err := metric.Update(); err != nil {
			fmt.Printf("Failed to update one-time metrics: %v\n", err)
			continue
		}
	}
}

// GetMetrics returns the metrics from all metrics
func (m *MetricsCollector) GetMetrics() map[string]interface{} {
	m.Mu.Lock()         // Acquire the lock before accessing shared resources
	defer m.Mu.Unlock() // Release the lock after the function returns

	dataMap := make(map[string]interface{})
	resData := make(map[string]([]map[string]map[string]string))
	for _, metric := range m.Metrics {
		monitorMetrics := metric.GetMetrics()
		res := getResourceConsumption(monitorMetrics)
		if _, ok := resData[metric.GetMetricsName()]; !ok {
			resData[metric.GetMetricsName()] = make([]map[string]map[string]string, 0)
		}
		resData[metric.GetMetricsName()] = append(resData[metric.GetMetricsName()], res)
	}

	for metricsName, metrics := range resData {
		dataMap[metricsName] = metrics
	}

	for _, metric := range m.OneTimeMetrics {
		oneTimeMetricsMap := make(map[string]interface{})
		monitorMetrics := metric.GetMetrics()
		for key, value := range monitorMetrics {
			oneTimeMetricsMap[key] = value
		}
		dataMap[metric.GetMetricsName()] = oneTimeMetricsMap
	}
	return dataMap
}

// Helper function to process Resource Consumption data
func getResourceConsumption(resourceMap map[string]string) map[string]map[string]string {
	result := map[string]map[string]string{}
	resourcePrefix := map[string]string{"MaxCPU": "Max CPU", "MaxMemory": "MaxMemory", "AverageCPU": "Average CPU", "AverageMemory": "Average Memory"}

	for key, value := range resourceMap {
		parts := strings.Split(key, ":")
		resourceName := parts[0]
		resourceType := parts[1]
		if _, ok := resourcePrefix[resourceType]; ok {
			if result[resourceName] == nil {
				result[resourceName] = map[string]string{}
			}
			result[resourceName][resourcePrefix[resourceType]] = value
		}
	}
	return result
}
