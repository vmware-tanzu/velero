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
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBackupMetricsWithAdhocBackups verifies that metrics are properly recorded
// for both scheduled and adhoc (non-scheduled) backups.
func TestBackupMetricsWithAdhocBackups(t *testing.T) {
	tests := []struct {
		name          string
		scheduleName  string
		expectedLabel string
		description   string
	}{
		{
			name:          "scheduled backup metrics",
			scheduleName:  "daily-backup",
			expectedLabel: "daily-backup",
			description:   "Metrics should be recorded with the schedule name label",
		},
		{
			name:          "adhoc backup metrics with empty schedule",
			scheduleName:  "",
			expectedLabel: "",
			description:   "Metrics should be recorded with empty schedule label for adhoc backups",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new metrics instance
			m := NewServerMetrics()

			// Test backup attempt metric
			t.Run("RegisterBackupAttempt", func(t *testing.T) {
				m.RegisterBackupAttempt(tc.scheduleName)

				metric := getMetricValue(t, m.metrics[backupAttemptTotal].(*prometheus.CounterVec), tc.expectedLabel)
				assert.Equal(t, float64(1), metric, tc.description)
			})

			// Test backup success metric
			t.Run("RegisterBackupSuccess", func(t *testing.T) {
				m.RegisterBackupSuccess(tc.scheduleName)

				metric := getMetricValue(t, m.metrics[backupSuccessTotal].(*prometheus.CounterVec), tc.expectedLabel)
				assert.Equal(t, float64(1), metric, tc.description)
			})

			// Test backup failure metric
			t.Run("RegisterBackupFailed", func(t *testing.T) {
				m.RegisterBackupFailed(tc.scheduleName)

				metric := getMetricValue(t, m.metrics[backupFailureTotal].(*prometheus.CounterVec), tc.expectedLabel)
				assert.Equal(t, float64(1), metric, tc.description)
			})

			// Test backup partial failure metric
			t.Run("RegisterBackupPartialFailure", func(t *testing.T) {
				m.RegisterBackupPartialFailure(tc.scheduleName)

				metric := getMetricValue(t, m.metrics[backupPartialFailureTotal].(*prometheus.CounterVec), tc.expectedLabel)
				assert.Equal(t, float64(1), metric, tc.description)
			})

			// Test backup validation failure metric
			t.Run("RegisterBackupValidationFailure", func(t *testing.T) {
				m.RegisterBackupValidationFailure(tc.scheduleName)

				metric := getMetricValue(t, m.metrics[backupValidationFailureTotal].(*prometheus.CounterVec), tc.expectedLabel)
				assert.Equal(t, float64(1), metric, tc.description)
			})

			// Test backup warning metric
			t.Run("RegisterBackupWarning", func(t *testing.T) {
				m.RegisterBackupWarning(tc.scheduleName)

				metric := getMetricValue(t, m.metrics[backupWarningTotal].(*prometheus.CounterVec), tc.expectedLabel)
				assert.Equal(t, float64(1), metric, tc.description)
			})

			// Test backup items total gauge
			t.Run("RegisterBackupItemsTotalGauge", func(t *testing.T) {
				m.RegisterBackupItemsTotalGauge(tc.scheduleName, 100)

				metric := getMetricValue(t, m.metrics[backupItemsTotalGauge].(*prometheus.GaugeVec), tc.expectedLabel)
				assert.Equal(t, float64(100), metric, tc.description)
			})

			// Test backup items errors gauge
			t.Run("RegisterBackupItemsErrorsGauge", func(t *testing.T) {
				m.RegisterBackupItemsErrorsGauge(tc.scheduleName, 5)

				metric := getMetricValue(t, m.metrics[backupItemsErrorsGauge].(*prometheus.GaugeVec), tc.expectedLabel)
				assert.Equal(t, float64(5), metric, tc.description)
			})

			// Test backup duration metric
			t.Run("RegisterBackupDuration", func(t *testing.T) {
				m.RegisterBackupDuration(tc.scheduleName, 120.5)

				// For histogram, we check the count
				metric := getHistogramCount(t, m.metrics[backupDurationSeconds].(*prometheus.HistogramVec), tc.expectedLabel)
				assert.Equal(t, uint64(1), metric, tc.description)
			})

			// Test backup last status metric
			t.Run("RegisterBackupLastStatus", func(t *testing.T) {
				m.RegisterBackupLastStatus(tc.scheduleName, BackupLastStatusSucc)

				metric := getMetricValue(t, m.metrics[backupLastStatus].(*prometheus.GaugeVec), tc.expectedLabel)
				assert.Equal(t, float64(BackupLastStatusSucc), metric, tc.description)
			})

			// Test backup tarball size metric
			t.Run("SetBackupTarballSizeBytesGauge", func(t *testing.T) {
				m.SetBackupTarballSizeBytesGauge(tc.scheduleName, 1024*1024)

				metric := getMetricValue(t, m.metrics[backupTarballSizeBytesGauge].(*prometheus.GaugeVec), tc.expectedLabel)
				assert.Equal(t, float64(1024*1024), metric, tc.description)
			})

			// Test backup last successful timestamp
			t.Run("SetBackupLastSuccessfulTimestamp", func(t *testing.T) {
				testTime := time.Now()
				m.SetBackupLastSuccessfulTimestamp(tc.scheduleName, testTime)

				metric := getMetricValue(t, m.metrics[backupLastSuccessfulTimestamp].(*prometheus.GaugeVec), tc.expectedLabel)
				assert.Equal(t, float64(testTime.Unix()), metric, tc.description)
			})
		})
	}
}

// TestRestoreMetricsWithAdhocBackups verifies that restore metrics are properly recorded
// for restores from both scheduled and adhoc backups.
func TestRestoreMetricsWithAdhocBackups(t *testing.T) {
	tests := []struct {
		name          string
		scheduleName  string
		expectedLabel string
		description   string
	}{
		{
			name:          "restore from scheduled backup",
			scheduleName:  "daily-backup",
			expectedLabel: "daily-backup",
			description:   "Restore metrics should use the backup's schedule name",
		},
		{
			name:          "restore from adhoc backup",
			scheduleName:  "",
			expectedLabel: "",
			description:   "Restore metrics should have empty schedule label for adhoc backup restores",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := NewServerMetrics()

			// Test restore attempt metric
			t.Run("RegisterRestoreAttempt", func(t *testing.T) {
				m.RegisterRestoreAttempt(tc.scheduleName)

				metric := getMetricValue(t, m.metrics[restoreAttemptTotal].(*prometheus.CounterVec), tc.expectedLabel)
				assert.Equal(t, float64(1), metric, tc.description)
			})

			// Test restore success metric
			t.Run("RegisterRestoreSuccess", func(t *testing.T) {
				m.RegisterRestoreSuccess(tc.scheduleName)

				metric := getMetricValue(t, m.metrics[restoreSuccessTotal].(*prometheus.CounterVec), tc.expectedLabel)
				assert.Equal(t, float64(1), metric, tc.description)
			})

			// Test restore failed metric
			t.Run("RegisterRestoreFailed", func(t *testing.T) {
				m.RegisterRestoreFailed(tc.scheduleName)

				metric := getMetricValue(t, m.metrics[restoreFailedTotal].(*prometheus.CounterVec), tc.expectedLabel)
				assert.Equal(t, float64(1), metric, tc.description)
			})

			// Test restore partial failure metric
			t.Run("RegisterRestorePartialFailure", func(t *testing.T) {
				m.RegisterRestorePartialFailure(tc.scheduleName)

				metric := getMetricValue(t, m.metrics[restorePartialFailureTotal].(*prometheus.CounterVec), tc.expectedLabel)
				assert.Equal(t, float64(1), metric, tc.description)
			})

			// Test restore validation failed metric
			t.Run("RegisterRestoreValidationFailed", func(t *testing.T) {
				m.RegisterRestoreValidationFailed(tc.scheduleName)

				metric := getMetricValue(t, m.metrics[restoreValidationFailedTotal].(*prometheus.CounterVec), tc.expectedLabel)
				assert.Equal(t, float64(1), metric, tc.description)
			})
		})
	}
}

// TestMultipleAdhocBackupsShareMetrics verifies that multiple adhoc backups
// accumulate metrics under the same empty schedule label.
func TestMultipleAdhocBackupsShareMetrics(t *testing.T) {
	m := NewServerMetrics()

	// Simulate multiple adhoc backup attempts
	for i := 0; i < 5; i++ {
		m.RegisterBackupAttempt("")
	}

	// Simulate some successes and failures
	m.RegisterBackupSuccess("")
	m.RegisterBackupSuccess("")
	m.RegisterBackupFailed("")
	m.RegisterBackupPartialFailure("")
	m.RegisterBackupValidationFailure("")

	// Verify accumulated metrics
	attemptMetric := getMetricValue(t, m.metrics[backupAttemptTotal].(*prometheus.CounterVec), "")
	assert.Equal(t, float64(5), attemptMetric, "All adhoc backup attempts should be counted together")

	successMetric := getMetricValue(t, m.metrics[backupSuccessTotal].(*prometheus.CounterVec), "")
	assert.Equal(t, float64(2), successMetric, "All adhoc backup successes should be counted together")

	failureMetric := getMetricValue(t, m.metrics[backupFailureTotal].(*prometheus.CounterVec), "")
	assert.Equal(t, float64(1), failureMetric, "All adhoc backup failures should be counted together")

	partialFailureMetric := getMetricValue(t, m.metrics[backupPartialFailureTotal].(*prometheus.CounterVec), "")
	assert.Equal(t, float64(1), partialFailureMetric, "All adhoc partial failures should be counted together")

	validationFailureMetric := getMetricValue(t, m.metrics[backupValidationFailureTotal].(*prometheus.CounterVec), "")
	assert.Equal(t, float64(1), validationFailureMetric, "All adhoc validation failures should be counted together")
}

// TestInitScheduleWithEmptyName verifies that InitSchedule works correctly
// with an empty schedule name (for adhoc backups).
func TestInitScheduleWithEmptyName(t *testing.T) {
	m := NewServerMetrics()

	// Initialize metrics for empty schedule (adhoc backups)
	m.InitSchedule("")

	// Verify all metrics are initialized with 0
	metrics := []string{
		backupAttemptTotal,
		backupSuccessTotal,
		backupPartialFailureTotal,
		backupFailureTotal,
		backupValidationFailureTotal,
		backupDeletionAttemptTotal,
		backupDeletionSuccessTotal,
		backupDeletionFailureTotal,
		backupItemsTotalGauge,
		backupItemsErrorsGauge,
		backupWarningTotal,
		restoreAttemptTotal,
		restorePartialFailureTotal,
		restoreFailedTotal,
		restoreSuccessTotal,
		restoreValidationFailedTotal,
		volumeSnapshotSuccessTotal,
		volumeSnapshotAttemptTotal,
		volumeSnapshotFailureTotal,
	}

	for _, metricName := range metrics {
		t.Run(metricName, func(t *testing.T) {
			var value float64
			switch vec := m.metrics[metricName].(type) {
			case *prometheus.CounterVec:
				value = getMetricValue(t, vec, "")
			case *prometheus.GaugeVec:
				value = getMetricValue(t, vec, "")
			}
			assert.Equal(t, float64(0), value, "Metric %s should be initialized to 0 for empty schedule", metricName)
		})
	}

	// Special case: backupLastStatus should be initialized to 1 (success)
	lastStatusValue := getMetricValue(t, m.metrics[backupLastStatus].(*prometheus.GaugeVec), "")
	assert.Equal(t, float64(1), lastStatusValue, "backupLastStatus should be initialized to 1 for empty schedule")
}

// Helper function to get metric value from a CounterVec or GaugeVec
func getMetricValue(t *testing.T, vec prometheus.Collector, scheduleLabel string) float64 {
	t.Helper()
	ch := make(chan prometheus.Metric, 1)
	vec.Collect(ch)
	close(ch)

	for metric := range ch {
		dto := &dto.Metric{}
		err := metric.Write(dto)
		require.NoError(t, err)

		// Check if this metric has the expected schedule label
		hasCorrectLabel := false
		for _, label := range dto.Label {
			if *label.Name == "schedule" && *label.Value == scheduleLabel {
				hasCorrectLabel = true
				break
			}
		}

		if hasCorrectLabel {
			if dto.Counter != nil {
				return *dto.Counter.Value
			}
			if dto.Gauge != nil {
				return *dto.Gauge.Value
			}
		}
	}

	t.Fatalf("Metric with schedule label '%s' not found", scheduleLabel)
	return 0
}

// Helper function to get histogram count
func getHistogramCount(t *testing.T, vec *prometheus.HistogramVec, scheduleLabel string) uint64 {
	t.Helper()
	ch := make(chan prometheus.Metric, 1)
	vec.Collect(ch)
	close(ch)

	for metric := range ch {
		dto := &dto.Metric{}
		err := metric.Write(dto)
		require.NoError(t, err)

		// Check if this metric has the expected schedule label
		hasCorrectLabel := false
		for _, label := range dto.Label {
			if *label.Name == "schedule" && *label.Value == scheduleLabel {
				hasCorrectLabel = true
				break
			}
		}

		if hasCorrectLabel && dto.Histogram != nil {
			return *dto.Histogram.SampleCount
		}
	}

	t.Fatalf("Histogram with schedule label '%s' not found", scheduleLabel)
	return 0
}
