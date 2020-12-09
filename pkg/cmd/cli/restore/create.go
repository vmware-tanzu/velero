/*
Copyright 2020 the Velero contributors.

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

package restore

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/util/flag"
	"github.com/vmware-tanzu/velero/pkg/cmd/util/output"
	veleroclient "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned"
	v1 "github.com/vmware-tanzu/velero/pkg/generated/informers/externalversions/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
)

func NewCreateCommand(f client.Factory, use string) *cobra.Command {
	o := NewCreateOptions()

	c := &cobra.Command{
		Use:   use + " [RESTORE_NAME] [--from-backup BACKUP_NAME | --from-schedule SCHEDULE_NAME]",
		Short: "Create a restore",
		Example: `  # Create a restore named "restore-1" from backup "backup-1".
  velero restore create restore-1 --from-backup backup-1

  # Create a restore with a default name ("backup-1-<timestamp>") from backup "backup-1".
  velero restore create --from-backup backup-1
 
  # Create a restore from the latest successful backup triggered by schedule "schedule-1".
  velero restore create --from-schedule schedule-1

  # Create a restore from the latest successful OR partially-failed backup triggered by schedule "schedule-1".
  velero restore create --from-schedule schedule-1 --allow-partially-failed

  # Create a restore for only persistentvolumeclaims and persistentvolumes within a backup.
  velero restore create --from-backup backup-2 --include-resources persistentvolumeclaims,persistentvolumes`,
		Args: cobra.MaximumNArgs(1),
		Run: func(c *cobra.Command, args []string) {
			cmd.CheckError(o.Complete(args, f))
			cmd.CheckError(o.Validate(c, args, f))
			cmd.CheckError(o.Run(c, f))
		},
	}

	o.BindFlags(c.Flags())
	output.BindFlags(c.Flags())
	output.ClearOutputFlagDefault(c)

	return c
}

type CreateOptions struct {
	BackupName              string
	ScheduleName            string
	RestoreName             string
	RestoreVolumes          flag.OptionalBool
	PreserveNodePorts       flag.OptionalBool
	Labels                  flag.Map
	IncludeNamespaces       flag.StringArray
	ExcludeNamespaces       flag.StringArray
	IncludeResources        flag.StringArray
	ExcludeResources        flag.StringArray
	NamespaceMappings       flag.Map
	Selector                flag.LabelSelector
	IncludeClusterResources flag.OptionalBool
	Wait                    bool
	AllowPartiallyFailed    flag.OptionalBool

	client veleroclient.Interface
}

func NewCreateOptions() *CreateOptions {
	return &CreateOptions{
		Labels:                  flag.NewMap(),
		IncludeNamespaces:       flag.NewStringArray("*"),
		NamespaceMappings:       flag.NewMap().WithEntryDelimiter(",").WithKeyValueDelimiter(":"),
		RestoreVolumes:          flag.NewOptionalBool(nil),
		PreserveNodePorts:       flag.NewOptionalBool(nil),
		IncludeClusterResources: flag.NewOptionalBool(nil),
	}
}

func (o *CreateOptions) BindFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.BackupName, "from-backup", "", "Backup to restore from")
	flags.StringVar(&o.ScheduleName, "from-schedule", "", "Schedule to restore from")
	flags.Var(&o.IncludeNamespaces, "include-namespaces", "Namespaces to include in the restore (use '*' for all namespaces)")
	flags.Var(&o.ExcludeNamespaces, "exclude-namespaces", "Namespaces to exclude from the restore.")
	flags.Var(&o.NamespaceMappings, "namespace-mappings", "Namespace mappings from name in the backup to desired restored name in the form src1:dst1,src2:dst2,...")
	flags.Var(&o.Labels, "labels", "Labels to apply to the restore.")
	flags.Var(&o.IncludeResources, "include-resources", "Resources to include in the restore, formatted as resource.group, such as storageclasses.storage.k8s.io (use '*' for all resources).")
	flags.Var(&o.ExcludeResources, "exclude-resources", "Resources to exclude from the restore, formatted as resource.group, such as storageclasses.storage.k8s.io.")
	flags.VarP(&o.Selector, "selector", "l", "Only restore resources matching this label selector.")
	f := flags.VarPF(&o.RestoreVolumes, "restore-volumes", "", "Whether to restore volumes from snapshots.")
	// this allows the user to just specify "--restore-volumes" as shorthand for "--restore-volumes=true"
	// like a normal bool flag
	f.NoOptDefVal = "true"

	f = flags.VarPF(&o.PreserveNodePorts, "preserve-nodeports", "", "Whether to preserve nodeports of Services when restoring.")
	// this allows the user to just specify "--preserve-nodeports" as shorthand for "--preserve-nodeports=true"
	// like a normal bool flag
	f.NoOptDefVal = "true"

	f = flags.VarPF(&o.IncludeClusterResources, "include-cluster-resources", "", "Include cluster-scoped resources in the restore.")
	f.NoOptDefVal = "true"

	f = flags.VarPF(&o.AllowPartiallyFailed, "allow-partially-failed", "", "If using --from-schedule, whether to consider PartiallyFailed backups when looking for the most recent one. This flag has no effect if not using --from-schedule.")
	f.NoOptDefVal = "true"

	flags.BoolVarP(&o.Wait, "wait", "w", o.Wait, "Wait for the operation to complete.")
}

func (o *CreateOptions) Complete(args []string, f client.Factory) error {
	if len(args) == 1 {
		o.RestoreName = args[0]
	} else {
		sourceName := o.BackupName
		if o.ScheduleName != "" {
			sourceName = o.ScheduleName
		}

		o.RestoreName = fmt.Sprintf("%s-%s", sourceName, time.Now().Format("20060102150405"))
	}

	client, err := f.Client()
	if err != nil {
		return err
	}
	o.client = client

	return nil
}

func (o *CreateOptions) Validate(c *cobra.Command, args []string, f client.Factory) error {
	if o.BackupName != "" && o.ScheduleName != "" {
		return errors.New("either a backup or schedule must be specified, but not both")
	}

	if o.BackupName == "" && o.ScheduleName == "" {
		return errors.New("either a backup or schedule must be specified, but not both")
	}

	if err := output.ValidateFlags(c); err != nil {
		return err
	}

	if o.client == nil {
		// This should never happen
		return errors.New("Velero client is not set; unable to proceed")
	}

	switch {
	case o.BackupName != "":
		if _, err := o.client.VeleroV1().Backups(f.Namespace()).Get(context.TODO(), o.BackupName, metav1.GetOptions{}); err != nil {
			return err
		}
	case o.ScheduleName != "":
		backupItems, err := o.client.VeleroV1().Backups(f.Namespace()).List(context.TODO(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", api.ScheduleNameLabel, o.ScheduleName)})
		if err != nil {
			return err
		}
		if len(backupItems.Items) == 0 {
			return errors.Errorf("No backups found for the schedule %s", o.ScheduleName)
		}
	}

	return nil
}

// mostRecentBackup returns the backup with the most recent start timestamp that has a phase that's
// in the provided list of allowed phases.
func mostRecentBackup(backups []api.Backup, allowedPhases ...api.BackupPhase) *api.Backup {
	// sort the backups in descending order of start time (i.e. most recent to least recent)
	sort.Slice(backups, func(i, j int) bool {
		// Use .After() because we want descending sort.

		var iStartTime, jStartTime time.Time
		if backups[i].Status.StartTimestamp != nil {
			iStartTime = backups[i].Status.StartTimestamp.Time
		}
		if backups[j].Status.StartTimestamp != nil {
			jStartTime = backups[j].Status.StartTimestamp.Time
		}
		return iStartTime.After(jStartTime)
	})

	// create a map of the allowed phases for easy lookup below
	phases := map[api.BackupPhase]struct{}{}
	for _, phase := range allowedPhases {
		phases[phase] = struct{}{}
	}

	var res *api.Backup
	for i, backup := range backups {
		// if the backup's phase is one of the allowable ones, record
		// the backup and break the loop so we can return it
		if _, ok := phases[backup.Status.Phase]; ok {
			res = &backups[i]
			break
		}
	}

	return res
}

func (o *CreateOptions) Run(c *cobra.Command, f client.Factory) error {
	if o.client == nil {
		// This should never happen
		return errors.New("Velero client is not set; unable to proceed")
	}

	// if --allow-partially-failed was specified, look up the most recent Completed or
	// PartiallyFailed backup for the provided schedule, and use that specific backup
	// to restore from.
	if o.ScheduleName != "" && boolptr.IsSetToTrue(o.AllowPartiallyFailed.Value) {
		backups, err := o.client.VeleroV1().Backups(f.Namespace()).List(context.TODO(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", api.ScheduleNameLabel, o.ScheduleName)})
		if err != nil {
			return err
		}

		// if we find a Completed or PartiallyFailed backup for the schedule, restore specifically from that backup. If we don't
		// find one, proceed as-is -- the Velero server will handle validation.
		if backup := mostRecentBackup(backups.Items, api.BackupPhaseCompleted, api.BackupPhasePartiallyFailed); backup != nil {
			// TODO(sk): this is kind of a hack -- we should revisit this and probably
			// move this logic to the server side or otherwise solve this problem.
			o.BackupName = backup.Name
			o.ScheduleName = ""
		}
	}

	restore := &api.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: f.Namespace(),
			Name:      o.RestoreName,
			Labels:    o.Labels.Data(),
		},
		Spec: api.RestoreSpec{
			BackupName:              o.BackupName,
			ScheduleName:            o.ScheduleName,
			IncludedNamespaces:      o.IncludeNamespaces,
			ExcludedNamespaces:      o.ExcludeNamespaces,
			IncludedResources:       o.IncludeResources,
			ExcludedResources:       o.ExcludeResources,
			NamespaceMapping:        o.NamespaceMappings.Data(),
			LabelSelector:           o.Selector.LabelSelector,
			RestorePVs:              o.RestoreVolumes.Value,
			PreserveNodePorts:       o.PreserveNodePorts.Value,
			IncludeClusterResources: o.IncludeClusterResources.Value,
		},
	}

	if printed, err := output.PrintWithFormat(c, restore); printed || err != nil {
		return err
	}

	var restoreInformer cache.SharedIndexInformer
	var updates chan *api.Restore
	if o.Wait {
		stop := make(chan struct{})
		defer close(stop)

		updates = make(chan *api.Restore)

		restoreInformer = v1.NewRestoreInformer(o.client, f.Namespace(), 0, nil)

		restoreInformer.AddEventHandler(
			cache.FilteringResourceEventHandler{
				FilterFunc: func(obj interface{}) bool {
					restore, ok := obj.(*api.Restore)
					if !ok {
						return false
					}
					return restore.Name == o.RestoreName
				},
				Handler: cache.ResourceEventHandlerFuncs{
					UpdateFunc: func(_, obj interface{}) {
						restore, ok := obj.(*api.Restore)
						if !ok {
							return
						}
						updates <- restore
					},
					DeleteFunc: func(obj interface{}) {
						restore, ok := obj.(*api.Restore)
						if !ok {
							return
						}
						updates <- restore
					},
				},
			},
		)
		go restoreInformer.Run(stop)
	}

	restore, err := o.client.VeleroV1().Restores(restore.Namespace).Create(context.TODO(), restore, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	fmt.Printf("Restore request %q submitted successfully.\n", restore.Name)
	if o.Wait {
		fmt.Println("Waiting for restore to complete. You may safely press ctrl-c to stop waiting - your restore will continue in the background.")
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				fmt.Print(".")
			case restore, ok := <-updates:
				if !ok {
					fmt.Println("\nError waiting: unable to watch restores.")
					return nil
				}

				if restore.Status.Phase != api.RestorePhaseNew && restore.Status.Phase != api.RestorePhaseInProgress {
					fmt.Printf("\nRestore completed with status: %s. You may check for more information using the commands `velero restore describe %s` and `velero restore logs %s`.\n", restore.Status.Phase, restore.Name, restore.Name)
					return nil
				}
			}
		}
	}

	// Not waiting

	fmt.Printf("Run `velero restore describe %s` or `velero restore logs %s` for more details.\n", restore.Name, restore.Name)

	return nil
}
