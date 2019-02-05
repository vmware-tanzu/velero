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

// ServerMetrics contains Prometheus metrics for the Velero server.
type ServerMetrics struct {
	metrics map[string]prometheus.Collector
}

const (
	metricNamespace              = "velero"
	backupTarballSizeBytesGauge  = "backup_tarball_size_bytes"
	backupAttemptTotal           = "backup_attempt_total"
	backupSuccessTotal           = "backup_success_total"
	backupFailureTotal           = "backup_failure_total"
	backupDurationSeconds        = "backup_duration_seconds"
	restoreAttemptTotal          = "restore_attempt_total"
	restoreValidationFailedTotal = "restore_validation_failed_total"
	restoreSuccessTotal          = "restore_success_total"
	restoreFailedTotal           = "restore_failed_total"
	volumeSnapshotAttemptTotal   = "volume_snapshot_attempt_total"
	volumeSnapshotSuccessTotal   = "volume_snapshot_success_total"
	volumeSnapshotFailureTotal   = "volume_snapshot_failure_total"

	scheduleLabel   = "schedule"
	backupNameLabel = "backupName"

	secondsInMinute = 60.0

	// -------------------------------------------------------------------
	// TODO: remove this code to remove the ark-namespaced metrics
	legacyMetricNamespace = "ark"

	// These variables are used only as the keys into a map; the standard variable names above are what is rendered for Prometheus
	// The Prometheus metric types themselves will take a namespace (ark or velero) and the metric name to construct the output that is scraped.
	legacyBackupTarballSizeBytesGauge  = "ark-backup_tarball_size_bytes"
	legacyBackupAttemptTotal           = "ark-backup_attempt_total"
	legacyBackupSuccessTotal           = "ark-backup_success_total"
	legacyBackupFailureTotal           = "ark-backup_failure_total"
	legacyBackupDurationSeconds        = "ark-backup_duration_seconds"
	legacyRestoreAttemptTotal          = "ark-restore_attempt_total"
	legacyRestoreValidationFailedTotal = "ark-restore_validation_failed_total"
	legacyRestoreSuccessTotal          = "ark-restore_success_total"
	legacyRestoreFailedTotal           = "ark-restore_failed_total"
	legacyVolumeSnapshotAttemptTotal   = "ark-volume_snapshot_attempt_total"
	legacyVolumeSnapshotSuccessTotal   = "ark-volume_snapshot_success_total"
	legacyVolumeSnapshotFailureTotal   = "ark-volume_snapshot_failure_total"
	// TODO: remove code above this comment
	// -------------------------------------------------------------------
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
			backupAttemptTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: metricNamespace,
					Name:      backupAttemptTotal,
					Help:      "Total number of attempted backups",
				},
				[]string{scheduleLabel},
			),
			backupSuccessTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: metricNamespace,
					Name:      backupSuccessTotal,
					Help:      "Total number of successful backups",
				},
				[]string{scheduleLabel},
			),
			backupFailureTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: metricNamespace,
					Name:      backupFailureTotal,
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
			restoreFailedTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: metricNamespace,
					Name:      restoreFailedTotal,
					Help:      "Total number of failed restores",
				},
				[]string{scheduleLabel},
			),
			restoreValidationFailedTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: metricNamespace,
					Name:      restoreValidationFailedTotal,
					Help:      "Total number of failed restores failing validations",
				},
				[]string{scheduleLabel},
			),
			volumeSnapshotAttemptTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: metricNamespace,
					Name:      volumeSnapshotAttemptTotal,
					Help:      "Total number of attempted volume snapshots",
				},
				[]string{scheduleLabel},
			),
			volumeSnapshotSuccessTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: metricNamespace,
					Name:      volumeSnapshotSuccessTotal,
					Help:      "Total number of successful volume snapshots",
				},
				[]string{scheduleLabel},
			),
			volumeSnapshotFailureTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: metricNamespace,
					Name:      volumeSnapshotFailureTotal,
					Help:      "Total number of failed volume snapshots",
				},
				[]string{scheduleLabel},
			),
			// -------------------------------------------------------------------
			// Ark backwards compatibility code
			// TODO: remove this code to drop the ark-namespaced metrics.
			legacyBackupTarballSizeBytesGauge: prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace: legacyMetricNamespace,
					Name:      backupTarballSizeBytesGauge,
					Help:      "Size, in bytes, of a backup",
				},
				[]string{scheduleLabel},
			),
			legacyBackupAttemptTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: legacyMetricNamespace,
					Name:      backupAttemptTotal,
					Help:      "Total number of attempted backups",
				},
				[]string{scheduleLabel},
			),
			legacyBackupSuccessTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: legacyMetricNamespace,
					Name:      backupSuccessTotal,
					Help:      "Total number of successful backups",
				},
				[]string{scheduleLabel},
			),
			legacyBackupFailureTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: legacyMetricNamespace,
					Name:      backupFailureTotal,
					Help:      "Total number of failed backups",
				},
				[]string{scheduleLabel},
			),
			legacyBackupDurationSeconds: prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Namespace: legacyMetricNamespace,
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
			legacyRestoreAttemptTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: legacyMetricNamespace,
					Name:      restoreAttemptTotal,
					Help:      "Total number of attempted restores",
				},
				[]string{scheduleLabel},
			),
			legacyRestoreSuccessTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: legacyMetricNamespace,
					Name:      restoreSuccessTotal,
					Help:      "Total number of successful restores",
				},
				[]string{scheduleLabel},
			),
			legacyRestoreFailedTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: legacyMetricNamespace,
					Name:      restoreFailedTotal,
					Help:      "Total number of failed restores",
				},
				[]string{scheduleLabel},
			),
			legacyRestoreValidationFailedTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: legacyMetricNamespace,
					Name:      restoreValidationFailedTotal,
					Help:      "Total number of failed restores failing validations",
				},
				[]string{scheduleLabel},
			),
			legacyVolumeSnapshotAttemptTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: legacyMetricNamespace,
					Name:      volumeSnapshotAttemptTotal,
					Help:      "Total number of attempted volume snapshots",
				},
				[]string{scheduleLabel},
			),
			legacyVolumeSnapshotSuccessTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: legacyMetricNamespace,
					Name:      volumeSnapshotSuccessTotal,
					Help:      "Total number of successful volume snapshots",
				},
				[]string{scheduleLabel},
			),
			legacyVolumeSnapshotFailureTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: legacyMetricNamespace,
					Name:      volumeSnapshotFailureTotal,
					Help:      "Total number of failed volume snapshots",
				},
				[]string{scheduleLabel},
			),
			// TODO: remove code above this comment
			// -------------------------------------------------------------------
		},
	}
}

// RegisterAllMetrics registers all prometheus metrics.
func (m *ServerMetrics) RegisterAllMetrics() {
	for _, pm := range m.metrics {
		prometheus.MustRegister(pm)
	}
}

// InitSchedule initializes counter metrics of a schedule.
func (m *ServerMetrics) InitSchedule(scheduleName string) {
	if c, ok := m.metrics[backupAttemptTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Set(0)
	}
	if c, ok := m.metrics[backupSuccessTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Set(0)
	}
	if c, ok := m.metrics[backupFailureTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Set(0)
	}
	if c, ok := m.metrics[restoreAttemptTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Set(0)
	}
	if c, ok := m.metrics[restoreFailedTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Set(0)
	}
	if c, ok := m.metrics[restoreSuccessTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Set(0)
	}
	if c, ok := m.metrics[restoreValidationFailedTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Set(0)
	}
	if c, ok := m.metrics[volumeSnapshotSuccessTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Set(0)
	}
	if c, ok := m.metrics[volumeSnapshotAttemptTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Set(0)
	}
	if c, ok := m.metrics[volumeSnapshotFailureTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Set(0)
	}

	// -------------------------------------------------------------------
	// TODO: remove this code to remove the ark-namespaced metrics
	if c, ok := m.metrics[legacyBackupAttemptTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Set(0)
	}
	if c, ok := m.metrics[legacyBackupSuccessTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Set(0)
	}
	if c, ok := m.metrics[legacyBackupFailureTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Set(0)
	}
	if c, ok := m.metrics[legacyRestoreAttemptTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Set(0)
	}
	if c, ok := m.metrics[legacyRestoreFailedTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Set(0)
	}
	if c, ok := m.metrics[legacyRestoreSuccessTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Set(0)
	}
	if c, ok := m.metrics[legacyRestoreValidationFailedTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Set(0)
	}
	if c, ok := m.metrics[legacyVolumeSnapshotSuccessTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Set(0)
	}
	if c, ok := m.metrics[legacyVolumeSnapshotAttemptTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Set(0)
	}
	if c, ok := m.metrics[legacyVolumeSnapshotFailureTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Set(0)
	}
	// TODO: remove code above this comment
	// -------------------------------------------------------------------
}

// SetBackupTarballSizeBytesGauge records the size, in bytes, of a backup tarball.
func (m *ServerMetrics) SetBackupTarballSizeBytesGauge(backupSchedule string, size int64) {
	if g, ok := m.metrics[backupTarballSizeBytesGauge].(*prometheus.GaugeVec); ok {
		g.WithLabelValues(backupSchedule).Set(float64(size))
	}
	// -------------------------------------------------------------------
	// TODO: remove this code to remove the ark-namespaced metrics
	if g, ok := m.metrics[legacyBackupTarballSizeBytesGauge].(*prometheus.GaugeVec); ok {
		g.WithLabelValues(backupSchedule).Set(float64(size))
	}
	// TODO: remove code above this comment
	// -------------------------------------------------------------------
}

// RegisterBackupAttempt records an backup attempt.
func (m *ServerMetrics) RegisterBackupAttempt(backupSchedule string) {
	if c, ok := m.metrics[backupAttemptTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
	// -------------------------------------------------------------------
	// TODO: remove this code to remove the ark-namespaced metrics
	if c, ok := m.metrics[legacyBackupAttemptTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
	// TODO: remove code above this comment
	// -------------------------------------------------------------------
}

// RegisterBackupSuccess records a successful completion of a backup.
func (m *ServerMetrics) RegisterBackupSuccess(backupSchedule string) {
	if c, ok := m.metrics[backupSuccessTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
	// -------------------------------------------------------------------
	// TODO: remove this code to remove the ark-namespaced metrics
	if c, ok := m.metrics[legacyBackupSuccessTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
	// TODO: remove code above this comment
	// -------------------------------------------------------------------
}

// RegisterBackupFailed records a failed backup.
func (m *ServerMetrics) RegisterBackupFailed(backupSchedule string) {
	if c, ok := m.metrics[backupFailureTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
	// -------------------------------------------------------------------
	// TODO: remove this code to remove the ark-namespaced metrics
	if c, ok := m.metrics[legacyBackupFailureTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
	// TODO: remove code above this comment
	// -------------------------------------------------------------------
}

// RegisterBackupDuration records the number of seconds a backup took.
func (m *ServerMetrics) RegisterBackupDuration(backupSchedule string, seconds float64) {
	if c, ok := m.metrics[backupDurationSeconds].(*prometheus.HistogramVec); ok {
		c.WithLabelValues(backupSchedule).Observe(seconds)
	}
	// -------------------------------------------------------------------
	// TODO: remove this code to remove the ark-namespaced metrics
	if c, ok := m.metrics[legacyBackupDurationSeconds].(*prometheus.HistogramVec); ok {
		c.WithLabelValues(backupSchedule).Observe(seconds)
	}
	// TODO: remove code above this comment
	// -------------------------------------------------------------------
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
	// -------------------------------------------------------------------
	// TODO: remove this code to remove the ark-namespaced metrics
	if c, ok := m.metrics[legacyRestoreAttemptTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
	// TODO: remove code above this comment
	// -------------------------------------------------------------------
}

// RegisterRestoreSuccess records a successful (maybe partial) completion of a restore.
func (m *ServerMetrics) RegisterRestoreSuccess(backupSchedule string) {
	if c, ok := m.metrics[restoreSuccessTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
	// -------------------------------------------------------------------
	// TODO: remove this code to remove the ark-namespaced metrics
	if c, ok := m.metrics[legacyRestoreSuccessTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
	// TODO: remove code above this comment
	// -------------------------------------------------------------------
}

// RegisterRestoreFailed records a restore that failed.
func (m *ServerMetrics) RegisterRestoreFailed(backupSchedule string) {
	if c, ok := m.metrics[restoreFailedTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
	// -------------------------------------------------------------------
	// TODO: remove this code to remove the ark-namespaced metrics
	if c, ok := m.metrics[legacyRestoreFailedTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
	// TODO: remove code above this comment
	// -------------------------------------------------------------------
}

// RegisterRestoreValidationFailed records a restore that failed validation.
func (m *ServerMetrics) RegisterRestoreValidationFailed(backupSchedule string) {
	if c, ok := m.metrics[restoreValidationFailedTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
	// -------------------------------------------------------------------
	// TODO: remove this code to remove the ark-namespaced metrics
	if c, ok := m.metrics[legacyRestoreValidationFailedTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
	// TODO: remove code above this comment
	// -------------------------------------------------------------------
}

// RegisterVolumeSnapshotAttempts records an attempt to snapshot a volume.
func (m *ServerMetrics) RegisterVolumeSnapshotAttempts(backupSchedule string, volumeSnapshotsAttempted int) {
	if c, ok := m.metrics[volumeSnapshotAttemptTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Add(float64(volumeSnapshotsAttempted))
	}
	// -------------------------------------------------------------------
	// TODO: remove this code to remove the ark-namespaced metrics
	if c, ok := m.metrics[legacyVolumeSnapshotAttemptTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Add(float64(volumeSnapshotsAttempted))
	}
	// TODO: remove code above this comment
	// -------------------------------------------------------------------
}

// RegisterVolumeSnapshotSuccesses records a completed volume snapshot.
func (m *ServerMetrics) RegisterVolumeSnapshotSuccesses(backupSchedule string, volumeSnapshotsCompleted int) {
	if c, ok := m.metrics[volumeSnapshotSuccessTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Add(float64(volumeSnapshotsCompleted))
	}
	// -------------------------------------------------------------------
	// TODO: remove this code to remove the ark-namespaced metrics
	if c, ok := m.metrics[legacyVolumeSnapshotSuccessTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Add(float64(volumeSnapshotsCompleted))
	}
	// TODO: remove code above this comment
	// -------------------------------------------------------------------
}

// RegisterVolumeSnapshotFailures records a failed volume snapshot.
func (m *ServerMetrics) RegisterVolumeSnapshotFailures(backupSchedule string, volumeSnapshotsFailed int) {
	if c, ok := m.metrics[volumeSnapshotFailureTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Add(float64(volumeSnapshotsFailed))
	}
	// -------------------------------------------------------------------
	// TODO: remove this code to remove the ark-namespaced metrics
	if c, ok := m.metrics[legacyVolumeSnapshotFailureTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Add(float64(volumeSnapshotsFailed))
	}
	// TODO: remove code above this comment
	// -------------------------------------------------------------------
}
