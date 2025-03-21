# Schedule Skip Immediately Config Design
## Abstract
When unpausing schedule, a backup could be due immediately.
New Schedules also create new backup immediately.

This design allows user to *skip **immediately due** backup run upon unpausing or schedule creation*.

## Background
Currently, the default behavior of schedule when `.Status.LastBackup` is nil or is due immediately after unpausing, a backup will be created. This may not be a desired by all users (https://github.com/vmware-tanzu/velero/issues/6517)

User want ability to skip the first immediately due backup when schedule is unpaused and or created.

If you create a schedule with cron "45 * * * *" and pause it at say the 43rd minute and then unpause it at say 50th minute, a backup gets triggered (since .Status.LastBackup is nil or >60min ago).

With this design, user can skip the first immediately due backup when schedule is unpaused and or created.

## Goals
- Add an option so user can when unpausing (when immediately due) or creating new schedule, to not create a backup immediately.

## Non Goals
- Changing the default behavior

## High-Level Design
Add a new field with to the schedule spec and as a new cli flags for install, server, schedule commands; allowing user to skip immediately due backup when unpausing or schedule creation.

If CLI flag is specified during schedule unpause, velero will update the schedule spec accordingly and override prior spec for `skipImmediately``.

## Detailed Design
### CLI Changes
`velero schedule unpause` will now take an optional bool flag `--skip-immediately` to allow user to override the behavior configured for velero server (see `velero server` below).

`velero schedule unpause schedule-1 --skip-immediately=false` will unpause the schedule but not skip the backup if due immediately from `Schedule.Status.LastBackup` timestamp. Backup will be run at the next cron schedule.

`velero schedule unpause schedule-1 --skip-immediately=true` will unpause the schedule and skip the backup if due immediately from `Schedule.Status.LastBackup` timestamp. Backup will also be run at the next cron schedule.

`velero schedule unpause schedule-1` will check `.spec.SkipImmediately` in the schedule to determine behavior. This field will default to false to maintain prior behavior.

`velero server` will add a new flag `--schedule-skip-immediately` to configure default value to patch new schedules created without the field. This flag will default to false to maintain prior behavior if not set.

`velero install` will add a new flag `--schedule-skip-immediately` to configure default value to patch new schedules created without the field. This flag will default to false to maintain prior behavior if not set.

### API Changes
`pkg/apis/velero/v1/schedule_types.go`
```diff
// ScheduleSpec defines the specification for a Velero schedule
type ScheduleSpec struct {
	// Template is the definition of the Backup to be run
	// on the provided schedule
	Template BackupSpec `json:"template"`

	// Schedule is a Cron expression defining when to run
	// the Backup.
	Schedule string `json:"schedule"`

	// UseOwnerReferencesBackup specifies whether to use
	// OwnerReferences on backups created by this Schedule.
	// +optional
	// +nullable
	UseOwnerReferencesInBackup *bool `json:"useOwnerReferencesInBackup,omitempty"`

	// Paused specifies whether the schedule is paused or not
	// +optional
	Paused bool `json:"paused,omitempty"`

+	// SkipImmediately specifies whether to skip backup if schedule is due immediately from `Schedule.Status.LastBackup` timestamp when schedule is unpaused or if schedule is new.
+	// If true, backup will be skipped immediately when schedule is unpaused if it is due based on .Status.LastBackupTimestamp or schedule is new, and will run at next schedule time.
+	// If false, backup will not be skipped immediately when schedule is unpaused, but will run at next schedule time.
+	// If empty, will follow server configuration (default: false). 
+	// +optional
+	SkipImmediately bool `json:"skipImmediately,omitempty"`
}
```

**Note:** The Velero server automatically patches the `skipImmediately` field back to `false` after it's been used. This is because `skipImmediately` is designed to be a one-time operation rather than a persistent state. When the controller detects that `skipImmediately` is set to `true`, it:
1. Sets the flag back to `false`
2. Records the current time in `schedule.Status.LastSkipped`

This "consume and reset" pattern ensures that after skipping one immediate backup, the schedule returns to normal behavior for subsequent runs. The `LastSkipped` timestamp is then used to determine when the next backup should run.

```go
// From pkg/controller/schedule_controller.go
if schedule.Spec.SkipImmediately != nil && *schedule.Spec.SkipImmediately { 
    *schedule.Spec.SkipImmediately = false 
    schedule.Status.LastSkipped = &metav1.Time{Time: c.clock.Now()} 
} 
```

`LastSkipped` will be added to `ScheduleStatus` struct to track the last time a schedule was skipped.
```diff
// ScheduleStatus captures the current state of a Velero schedule
type ScheduleStatus struct {
	// Phase is the current phase of the Schedule
	// +optional
	Phase SchedulePhase `json:"phase,omitempty"`

	// LastBackup is the last time a Backup was run for this
	// Schedule schedule
	// +optional
	// +nullable
	LastBackup *metav1.Time `json:"lastBackup,omitempty"`

+	// LastSkipped is the last time a Schedule was skipped
+	// +optional
+	// +nullable
+	LastSkipped *metav1.Time `json:"lastSkipped,omitempty"`

	// ValidationErrors is a slice of all validation errors (if
	// applicable)
	// +optional
	ValidationErrors []string `json:"validationErrors,omitempty"`
}
```

The `LastSkipped` field is crucial for the schedule controller to determine the next run time. When a backup is skipped, this timestamp is used instead of `LastBackup` to calculate when the next backup should occur, ensuring the schedule maintains its intended cadence even after skipping a backup.

When `schedule.spec.SkipImmediately` is `true`, `LastSkipped` will be set to the current time, and `schedule.spec.SkipImmediately` set to nil so it can be used again.

The `getNextRunTime()` function below is updated so `LastSkipped` which is after `LastBackup` will be used to determine next run time.

```go
func getNextRunTime(schedule *velerov1.Schedule, cronSchedule cron.Schedule, asOf time.Time) (bool, time.Time) {
	var lastBackupTime time.Time
	if schedule.Status.LastBackup != nil {
		lastBackupTime = schedule.Status.LastBackup.Time
	} else {
		lastBackupTime = schedule.CreationTimestamp.Time
	}
	if schedule.Status.LastSkipped != nil && schedule.Status.LastSkipped.After(lastBackupTime) {
		lastBackupTime = schedule.Status.LastSkipped.Time
	}

	nextRunTime := cronSchedule.Next(lastBackupTime)

	return asOf.After(nextRunTime), nextRunTime
}
```

When schedule is unpaused, and `Schedule.Status.LastBackup` is not nil, if `Schedule.Status.LastSkipped` is recent, a backup will not be created.

When schedule is unpaused or created with `Schedule.Status.LastBackup` set to nil or schedule is newly created, normally a backup will be created immediately. If `Schedule.Status.LastSkipped` is recent, a backup will not be created.

Backup will be run at the next cron schedule based on LastBackup or LastSkipped whichever is more recent.

## Alternatives Considered

N/A


## Security Considerations
None

## Compatibility
Upon upgrade, the new field will be added to the schedule spec automatically and will default to the prior behavior of running a backup when schedule is unpaused if it is due based on .Status.LastBackup or schedule is new.

Since this is a new field, it will be ignored by older versions of velero.

## Implementation
TBD

## Open Issues
N/A
