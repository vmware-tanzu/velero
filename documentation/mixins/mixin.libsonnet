// based on Prometheus alert by GitHub user raqqun:
// https://github.com/vmware-tanzu/velero/issues/2725#issuecomment-661577500
{
  _config+:: {
    selector: '',
  },

  prometheusAlerts+:: {
    groups+: [
      {
        name: 'velero',
        rules: [
          {
            alert: 'VeleroBackupPartialFailures',
            expr: |||
              sum_over_time(velero_backup_partial_failure_total{schedule!="", %(selector)s}[7d]) / sum_over_time(velero_backup_attempt_total{schedule!="", %(selector)s}[7d]) > 0.25
            ||| % $._config,
            'for': '15m',
            labels: {
              severity: 'warning',
            },
            annotations: {
              description: 'Velero backup {{ $labels.schedule }} has {{ $value | humanizePercentage }} partially failed backups.',
              summary: 'Velero backup {{ $labels.schedule }} has too many partially failed backups',
            },
          },
          {
            alert: 'VeleroBackupFailures',
            expr: |||
              sum_over_time(velero_backup_failure_total{schedule!="", %(selector)s}[7d]) / sum_over_time(velero_backup_attempt_total{schedule!="", %(selector)s}[7d]) > 0.25
            ||| % $._config,
            'for': '15m',
            labels: {
              severity: 'warning',
            },
            annotations: {
              description: 'Velero backup {{ $labels.schedule }} has {{ $value | humanizePercentage }} failed backups.',
              summary: 'Velero backup {{ $labels.schedule }} has too many failed backups',
            },
          },
          {
            alert: 'VeleroUnsuccessfulBackup',
            expr: |||
              changes(velero_backup_last_successful_timestamp{schedule!~".*weekly.*"}[25h]) < 0
            ||| % $._config,
            'for': '15m',
            labels: {
              severity: 'warning',
            },
            annotations: {
              description: 'Velero backup was not successful for {{ $labels.schedule }}.',
              summary: 'Velero backup for schedule {{ $labels.schedule }} was unsuccessful.',
            },
          },
        ],
      },
    ],
  },
}
