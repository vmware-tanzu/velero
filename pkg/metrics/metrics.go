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

package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// ServerMetrics contains Prometheus metrics for the Ark server.
type ServerMetrics struct {
	metrics map[string]prometheus.Collector
}

const (
	metricNamespace             = "ark"
	backupTarballSizeBytesGauge = "backup_tarball_size_bytes"
	// TODO: Rename the Count variables to match their strings
	backupAttemptCount           = "backup_attempt_total"
	backupSuccessCount           = "backup_success_total"
	backupFailureCount           = "backup_failure_total"
	backupDurationSeconds        = "backup_duration_seconds"
	restoreAttemptTotal          = "restore_attempt_total"
	restoreValidationFailedTotal = "restore_validation_failed_total"
	restoreSuccessTotal          = "restore_success_total"
	restoreIncompleteTotal       = "restore_incomplete_total"

	scheduleLabel   = "schedule"
	backupNameLabel = "backupName"

	secondsInMinute = 60.0
)

// NewServerMetrics returns new ServerMetrics
func NewServerMetrics() *ServerMetrics {
	return &ServerMetrics{
		metrics: map[string]prometheus.Collector{
			backupTarballSizeBytesGauge: prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace: metricNamespace,
					Name:      backupTarballSizeBytesGauge,
					Help:      "Size, in bytes, of a backup",
				},
				[]string{scheduleLabel},
			),
			backupAttemptCount: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: metricNamespace,
					Name:      backupAttemptCount,
					Help:      "Total number of attempted backups",
				},
				[]string{scheduleLabel},
			),
			backupSuccessCount: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: metricNamespace,
					Name:      backupSuccessCount,
					Help:      "Total number of successful backups",
				},
				[]string{scheduleLabel},
			),
			backupFailureCount: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: metricNamespace,
					Name:      backupFailureCount,
					Help:      "Total number of failed backups",
				},
				[]string{scheduleLabel},
			),
			backupDurationSeconds: prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Namespace: metricNamespace,
					Name:      backupDurationSeconds,
					Help:      "Time taken to complete backup, in seconds",
					Buckets: []float64{
						toSeconds(1 * time.Minute),
						toSeconds(5 * time.Minute),
						toSeconds(10 * time.Minute),
						toSeconds(15 * time.Minute),
						toSeconds(30 * time.Minute),
						toSeconds(1 * time.Hour),
						toSeconds(2 * time.Hour),
						toSeconds(3 * time.Hour),
						toSeconds(4 * time.Hour),
					},
				},
				[]string{scheduleLabel},
			),
			restoreAttemptTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: metricNamespace,
					Name:      restoreAttemptTotal,
					Help:      "Total number of attempted restores",
				},
				[]string{scheduleLabel},
			),
			restoreSuccessTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: metricNamespace,
					Name:      restoreSuccessTotal,
					Help:      "Total number of successful restores",
				},
				[]string{scheduleLabel},
			),
			restoreIncompleteTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: metricNamespace,
					Name:      restoreIncompleteTotal,
					Help:      "Total number of incomplete restores",
				},
				[]string{scheduleLabel},
			),
			restoreValidationFailedTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: metricNamespace,
					Name:      restoreValidationFailedTotal,
					Help:      "Total number of failed restore validations",
				},
				[]string{scheduleLabel},
			),
		},
	}
}

// RegisterAllMetrics registers all prometheus metrics.
func (m *ServerMetrics) RegisterAllMetrics() {
	for _, pm := range m.metrics {
		prometheus.MustRegister(pm)
	}
}

func (m *ServerMetrics) InitSchedule(scheduleName string) {
	if c, ok := m.metrics[backupAttemptCount].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Set(0)
	}
	if c, ok := m.metrics[backupSuccessCount].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Set(0)
	}
	if c, ok := m.metrics[backupFailureCount].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Set(0)
	}
}

// SetBackupTarballSizeBytesGauge records the size, in bytes, of a backup tarball.
func (m *ServerMetrics) SetBackupTarballSizeBytesGauge(backupSchedule string, size int64) {
	if g, ok := m.metrics[backupTarballSizeBytesGauge].(*prometheus.GaugeVec); ok {
		g.WithLabelValues(backupSchedule).Set(float64(size))
	}
}

// RegisterBackupAttempt records an backup attempt.
func (m *ServerMetrics) RegisterBackupAttempt(backupSchedule string) {
	if c, ok := m.metrics[backupAttemptCount].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
}

// RegisterBackupSuccess records a successful completion of a backup.
func (m *ServerMetrics) RegisterBackupSuccess(backupSchedule string) {
	if c, ok := m.metrics[backupSuccessCount].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
}

// RegisterBackupFailed records a failed backup.
func (m *ServerMetrics) RegisterBackupFailed(backupSchedule string) {
	if c, ok := m.metrics[backupFailureCount].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
}

// RegisterBackupDuration records the number of seconds a backup took.
func (m *ServerMetrics) RegisterBackupDuration(backupSchedule string, seconds float64) {
	if c, ok := m.metrics[backupDurationSeconds].(*prometheus.HistogramVec); ok {
		c.WithLabelValues(backupSchedule).Observe(seconds)
	}
}

// toSeconds translates a time.Duration value into a float64
// representing the number of seconds in that duration.
func toSeconds(d time.Duration) float64 {
	return float64(d / time.Second)
}

// RegisterRestoreAttempt records an attempt to restore a backup.
func (m *ServerMetrics) RegisterRestoreAttempt(backupSchedule string) {
	if c, ok := m.metrics[restoreAttemptTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
}

// RegisterRestoreSuccess records a successful completion of a restore.
func (m *ServerMetrics) RegisterRestoreSuccess(backupSchedule string) {
	if c, ok := m.metrics[restoreSuccessTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
}

// RegisterRestoreIncomplete records a restore that finished with errors.
func (m *ServerMetrics) RegisterRestoreIncomplete(backupSchedule string) {
	if c, ok := m.metrics[restoreIncompleteTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
}

// RegisterRestoreValidationFailed records a failed restore.
func (m *ServerMetrics) RegisterRestoreValidationFailed(backupSchedule string) {
	if c, ok := m.metrics[restoreValidationFailedTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
}
