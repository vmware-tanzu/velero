/*
Copyright 2018 the Velero contributors.

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
	metricNamespace        = "velero"
	resticMetricsNamespace = "restic"
	//Velero metrics
	backupTarballSizeBytesGauge   = "backup_tarball_size_bytes"
	backupTotal                   = "backup_total"
	backupAttemptTotal            = "backup_attempt_total"
	backupSuccessTotal            = "backup_success_total"
	backupPartialFailureTotal     = "backup_partial_failure_total"
	backupFailureTotal            = "backup_failure_total"
	backupValidationFailureTotal  = "backup_validation_failure_total"
	backupDurationSeconds         = "backup_duration_seconds"
	backupDeletionAttemptTotal    = "backup_deletion_attempt_total"
	backupDeletionSuccessTotal    = "backup_deletion_success_total"
	backupDeletionFailureTotal    = "backup_deletion_failure_total"
	backupLastSuccessfulTimestamp = "backup_last_successful_timestamp"
	restoreTotal                  = "restore_total"
	restoreAttemptTotal           = "restore_attempt_total"
	restoreValidationFailedTotal  = "restore_validation_failed_total"
	restoreSuccessTotal           = "restore_success_total"
	restorePartialFailureTotal    = "restore_partial_failure_total"
	restoreFailedTotal            = "restore_failed_total"
	volumeSnapshotAttemptTotal    = "volume_snapshot_attempt_total"
	volumeSnapshotSuccessTotal    = "volume_snapshot_success_total"
	volumeSnapshotFailureTotal    = "volume_snapshot_failure_total"

	// Restic metrics
	podVolumeBackupEnqueueTotal        = "pod_volume_backup_enqueue_count"
	podVolumeBackupDequeueTotal        = "pod_volume_backup_dequeue_count"
	resticOperationLatencySeconds      = "restic_operation_latency_seconds"
	resticOperationLatencyGaugeSeconds = "restic_operation_latency_seconds_gauge"

	// Labels
	nodeMetricLabel      = "node"
	resticOperationLabel = "operation"
	pvbNameLabel         = "pod_volume_backup"
	scheduleLabel        = "schedule"
	backupNameLabel      = "backupName"

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
			backupLastSuccessfulTimestamp: prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace: metricNamespace,
					Name:      backupLastSuccessfulTimestamp,
					Help:      "Last time a backup ran successfully, Unix timestamp in seconds",
				},
				[]string{scheduleLabel},
			),
			backupTotal: prometheus.NewGauge(
				prometheus.GaugeOpts{
					Namespace: metricNamespace,
					Name:      backupTotal,
					Help:      "Current number of existent backups",
				},
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
			backupPartialFailureTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: metricNamespace,
					Name:      backupPartialFailureTotal,
					Help:      "Total number of partially failed backups",
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
			backupValidationFailureTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: metricNamespace,
					Name:      backupValidationFailureTotal,
					Help:      "Total number of validation failed backups",
				},
				[]string{scheduleLabel},
			),
			backupDeletionAttemptTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: metricNamespace,
					Name:      backupDeletionAttemptTotal,
					Help:      "Total number of attempted backup deletions",
				},
				[]string{scheduleLabel},
			),
			backupDeletionSuccessTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: metricNamespace,
					Name:      backupDeletionSuccessTotal,
					Help:      "Total number of successful backup deletions",
				},
				[]string{scheduleLabel},
			),
			backupDeletionFailureTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: metricNamespace,
					Name:      backupDeletionFailureTotal,
					Help:      "Total number of failed backup deletions",
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
			restoreTotal: prometheus.NewGauge(
				prometheus.GaugeOpts{
					Namespace: metricNamespace,
					Name:      restoreTotal,
					Help:      "Current number of existent restores",
				},
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
			restorePartialFailureTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: metricNamespace,
					Name:      restorePartialFailureTotal,
					Help:      "Total number of partially failed restores",
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
		},
	}
}

func NewResticServerMetrics() *ServerMetrics {
	return &ServerMetrics{
		metrics: map[string]prometheus.Collector{
			podVolumeBackupEnqueueTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: resticMetricsNamespace,
					Name:      podVolumeBackupEnqueueTotal,
					Help:      "Total number of pod_volume_backup objects enqueued",
				},
				[]string{nodeMetricLabel},
			),
			podVolumeBackupDequeueTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: resticMetricsNamespace,
					Name:      podVolumeBackupDequeueTotal,
					Help:      "Total number of pod_volume_backup objects dequeued",
				},
				[]string{nodeMetricLabel},
			),
			resticOperationLatencyGaugeSeconds: prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace: resticMetricsNamespace,
					Name:      resticOperationLatencyGaugeSeconds,
					Help:      "Gauge metric indicating time taken, in seconds, to perform restic operations",
				},
				[]string{nodeMetricLabel, resticOperationLabel, backupNameLabel, pvbNameLabel},
			),
			resticOperationLatencySeconds: prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Namespace: resticMetricsNamespace,
					Name:      resticOperationLatencySeconds,
					Help:      "Time taken to complete restic operations, in seconds",
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
				[]string{nodeMetricLabel, resticOperationLabel, backupNameLabel, pvbNameLabel},
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

// InitSchedule initializes counter metrics of a schedule.
func (m *ServerMetrics) InitSchedule(scheduleName string) {
	if c, ok := m.metrics[backupAttemptTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Add(0)
	}
	if c, ok := m.metrics[backupSuccessTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Add(0)
	}
	if c, ok := m.metrics[backupPartialFailureTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Add(0)
	}
	if c, ok := m.metrics[backupFailureTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Add(0)
	}
	if c, ok := m.metrics[backupValidationFailureTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Add(0)
	}
	if c, ok := m.metrics[backupDeletionAttemptTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Add(0)
	}
	if c, ok := m.metrics[backupDeletionSuccessTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Add(0)
	}
	if c, ok := m.metrics[backupDeletionFailureTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Add(0)
	}
	if c, ok := m.metrics[restoreAttemptTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Add(0)
	}
	if c, ok := m.metrics[restorePartialFailureTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Add(0)
	}
	if c, ok := m.metrics[restoreFailedTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Add(0)
	}
	if c, ok := m.metrics[restoreSuccessTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Add(0)
	}
	if c, ok := m.metrics[restoreValidationFailedTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Add(0)
	}
	if c, ok := m.metrics[volumeSnapshotSuccessTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Add(0)
	}
	if c, ok := m.metrics[volumeSnapshotAttemptTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Add(0)
	}
	if c, ok := m.metrics[volumeSnapshotFailureTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(scheduleName).Add(0)
	}
}

// InitSchedule initializes counter metrics for a node.
func (m *ServerMetrics) InitResticMetricsForNode(node string) {
	if c, ok := m.metrics[podVolumeBackupEnqueueTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(node).Add(0)
	}
	if c, ok := m.metrics[podVolumeBackupDequeueTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(node).Add(0)
	}
}

// RegisterPodVolumeBackupEnqueue records enqueuing of a PodVolumeBackup object.
func (m *ServerMetrics) RegisterPodVolumeBackupEnqueue(node string) {
	if c, ok := m.metrics[podVolumeBackupEnqueueTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(node).Inc()
	}
}

// RegisterPodVolumeBackupDequeue records dequeuing of a PodVolumeBackup object.
func (m *ServerMetrics) RegisterPodVolumeBackupDequeue(node string) {
	if c, ok := m.metrics[podVolumeBackupDequeueTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(node).Inc()
	}
}

// ObserveResticOpLatency records the number of seconds a restic operation took.
func (m *ServerMetrics) ObserveResticOpLatency(node, pvbName, opName, backupName string, seconds float64) {
	if h, ok := m.metrics[resticOperationLatencySeconds].(*prometheus.HistogramVec); ok {
		h.WithLabelValues(node, opName, backupName, pvbName).Observe(seconds)
	}
}

// RegisterResticOpLatencyGauge registers the restic operation latency as a gauge metric.
func (m *ServerMetrics) RegisterResticOpLatencyGauge(node, pvbName, opName, backupName string, seconds float64) {
	if g, ok := m.metrics[resticOperationLatencyGaugeSeconds].(*prometheus.GaugeVec); ok {
		g.WithLabelValues(node, opName, backupName, pvbName).Set(seconds)
	}
}

// SetBackupTarballSizeBytesGauge records the size, in bytes, of a backup tarball.
func (m *ServerMetrics) SetBackupTarballSizeBytesGauge(backupSchedule string, size int64) {
	if g, ok := m.metrics[backupTarballSizeBytesGauge].(*prometheus.GaugeVec); ok {
		g.WithLabelValues(backupSchedule).Set(float64(size))
	}
}

// SetBackupLastSuccessfulTimestamp records the last time a backup ran successfully, Unix timestamp in seconds
func (m *ServerMetrics) SetBackupLastSuccessfulTimestamp(backupSchedule string, time time.Time) {
	if g, ok := m.metrics[backupLastSuccessfulTimestamp].(*prometheus.GaugeVec); ok {
		g.WithLabelValues(backupSchedule).Set(float64(time.Unix()))
	}
}

// SetBackupTotal records the current number of existent backups.
func (m *ServerMetrics) SetBackupTotal(numberOfBackups int64) {
	if g, ok := m.metrics[backupTotal].(prometheus.Gauge); ok {
		g.Set(float64(numberOfBackups))
	}
}

// RegisterBackupAttempt records an backup attempt.
func (m *ServerMetrics) RegisterBackupAttempt(backupSchedule string) {
	if c, ok := m.metrics[backupAttemptTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
}

// RegisterBackupSuccess records a successful completion of a backup.
func (m *ServerMetrics) RegisterBackupSuccess(backupSchedule string) {
	if c, ok := m.metrics[backupSuccessTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
	m.SetBackupLastSuccessfulTimestamp(backupSchedule, time.Now())
}

// RegisterBackupPartialFailure records a partially failed backup.
func (m *ServerMetrics) RegisterBackupPartialFailure(backupSchedule string) {
	if c, ok := m.metrics[backupPartialFailureTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
}

// RegisterBackupFailed records a failed backup.
func (m *ServerMetrics) RegisterBackupFailed(backupSchedule string) {
	if c, ok := m.metrics[backupFailureTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
}

// RegisterBackupValidationFailure records a validation failed backup.
func (m *ServerMetrics) RegisterBackupValidationFailure(backupSchedule string) {
	if c, ok := m.metrics[backupValidationFailureTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
}

// RegisterBackupDuration records the number of seconds a backup took.
func (m *ServerMetrics) RegisterBackupDuration(backupSchedule string, seconds float64) {
	if c, ok := m.metrics[backupDurationSeconds].(*prometheus.HistogramVec); ok {
		c.WithLabelValues(backupSchedule).Observe(seconds)
	}
}

// RegisterBackupDeletionAttempt records the number of attempted backup deletions
func (m *ServerMetrics) RegisterBackupDeletionAttempt(backupSchedule string) {
	if c, ok := m.metrics[backupDeletionAttemptTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
}

// RegisterBackupDeletionFailed records the number of failed backup deletions
func (m *ServerMetrics) RegisterBackupDeletionFailed(backupSchedule string) {
	if c, ok := m.metrics[backupDeletionFailureTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
}

// RegisterBackupDeletionSuccess records the number of successful backup deletions
func (m *ServerMetrics) RegisterBackupDeletionSuccess(backupSchedule string) {
	if c, ok := m.metrics[backupDeletionSuccessTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
}

// toSeconds translates a time.Duration value into a float64
// representing the number of seconds in that duration.
func toSeconds(d time.Duration) float64 {
	return float64(d / time.Second)
}

// SetRestoreTotal records the current number of existent restores.
func (m *ServerMetrics) SetRestoreTotal(numberOfRestores int64) {
	if g, ok := m.metrics[restoreTotal].(prometheus.Gauge); ok {
		g.Set(float64(numberOfRestores))
	}
}

// RegisterRestoreAttempt records an attempt to restore a backup.
func (m *ServerMetrics) RegisterRestoreAttempt(backupSchedule string) {
	if c, ok := m.metrics[restoreAttemptTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
}

// RegisterRestoreSuccess records a successful (maybe partial) completion of a restore.
func (m *ServerMetrics) RegisterRestoreSuccess(backupSchedule string) {
	if c, ok := m.metrics[restoreSuccessTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
}

// RegisterRestorePartialFailure records a restore that partially failed.
func (m *ServerMetrics) RegisterRestorePartialFailure(backupSchedule string) {
	if c, ok := m.metrics[restorePartialFailureTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
}

// RegisterRestoreFailed records a restore that failed.
func (m *ServerMetrics) RegisterRestoreFailed(backupSchedule string) {
	if c, ok := m.metrics[restoreFailedTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
}

// RegisterRestoreValidationFailed records a restore that failed validation.
func (m *ServerMetrics) RegisterRestoreValidationFailed(backupSchedule string) {
	if c, ok := m.metrics[restoreValidationFailedTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
}

// RegisterVolumeSnapshotAttempts records an attempt to snapshot a volume.
func (m *ServerMetrics) RegisterVolumeSnapshotAttempts(backupSchedule string, volumeSnapshotsAttempted int) {
	if c, ok := m.metrics[volumeSnapshotAttemptTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Add(float64(volumeSnapshotsAttempted))
	}
}

// RegisterVolumeSnapshotSuccesses records a completed volume snapshot.
func (m *ServerMetrics) RegisterVolumeSnapshotSuccesses(backupSchedule string, volumeSnapshotsCompleted int) {
	if c, ok := m.metrics[volumeSnapshotSuccessTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Add(float64(volumeSnapshotsCompleted))
	}
}

// RegisterVolumeSnapshotFailures records a failed volume snapshot.
func (m *ServerMetrics) RegisterVolumeSnapshotFailures(backupSchedule string, volumeSnapshotsFailed int) {
	if c, ok := m.metrics[volumeSnapshotFailureTotal].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Add(float64(volumeSnapshotsFailed))
	}
}
