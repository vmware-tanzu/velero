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
	"github.com/prometheus/client_golang/prometheus"
)

// ServerMetrics contains Prometheus metrics for the Ark server.
type ServerMetrics struct {
	metrics map[string]prometheus.Collector
}

const (
	metricNamespace             = "ark"
	backupTarballSizeBytesGauge = "backup_tarball_size_bytes"
	backupAttemptCount          = "backup_attempt_total"
	backupSuccessCount          = "backup_success_total"
	backupFailureCount          = "backup_failure_total"

	scheduleLabel = "schedule"
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
		},
	}
}

// RegisterAllMetrics registers all prometheus metrics
func (m *ServerMetrics) RegisterAllMetrics() {
	for _, pm := range m.metrics {
		prometheus.MustRegister(pm)
	}
}

// SetBackupTarballSizeBytesGauge records the size, in bytes, of a backup tarball
func (m *ServerMetrics) SetBackupTarballSizeBytesGauge(backupSchedule string, size int64) {
	if g, ok := m.metrics[backupTarballSizeBytesGauge].(*prometheus.GaugeVec); ok {
		g.WithLabelValues(backupSchedule).Set(float64(size))
	}
}

// RegisterBackupAttempt records an backup attempt
func (m *ServerMetrics) RegisterBackupAttempt(backupSchedule string) {
	if c, ok := m.metrics[backupAttemptCount].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
}

// RegisterBackupSuccess records a successful completion of a backup
func (m *ServerMetrics) RegisterBackupSuccess(backupSchedule string) {
	if c, ok := m.metrics[backupSuccessCount].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
}

// RegisterBackupFailed records a failed backup
func (m *ServerMetrics) RegisterBackupFailed(backupSchedule string) {
	if c, ok := m.metrics[backupFailureCount].(*prometheus.CounterVec); ok {
		c.WithLabelValues(backupSchedule).Inc()
	}
}
