/*
Copyright 2017 Heptio Inc.

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

package schedule

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cmd"
	"github.com/heptio/ark/pkg/cmd/cli/backup"
	"github.com/heptio/ark/pkg/cmd/util/output"
)

func NewCreateCommand(f client.Factory, use string) *cobra.Command {
	o := NewCreateOptions()

	c := &cobra.Command{
		Use:   use + " NAME",
		Short: "Create a schedule",
		Run: func(c *cobra.Command, args []string) {
			cmd.CheckError(o.Validate(c, args))
			cmd.CheckError(o.Complete(args))
			cmd.CheckError(o.Run(c, f))
		},
	}

	o.BindFlags(c.Flags())
	output.BindFlags(c.Flags())
	output.ClearOutputFlagDefault(c)

	return c
}

type CreateOptions struct {
	BackupOptions *backup.CreateOptions
	Schedule      string

	labelSelector *metav1.LabelSelector
}

func NewCreateOptions() *CreateOptions {
	return &CreateOptions{
		BackupOptions: backup.NewCreateOptions(),
	}
}

func (o *CreateOptions) BindFlags(flags *pflag.FlagSet) {
	o.BackupOptions.BindFlags(flags)
	flags.StringVar(&o.Schedule, "schedule", o.Schedule, "a cron expression specifying a recurring schedule for this backup to run")
}

func (o *CreateOptions) Validate(c *cobra.Command, args []string) error {
	if len(args) != 1 {
		return errors.New("you must specify only one argument, the schedule's name")
	}
	if len(o.Schedule) == 0 {
		return errors.New("--schedule is required")
	}

	return o.BackupOptions.Validate(c, args)
}

func (o *CreateOptions) Complete(args []string) error {
	return o.BackupOptions.Complete(args)
}

func (o *CreateOptions) Run(c *cobra.Command, f client.Factory) error {
	arkClient, err := f.Client()
	if err != nil {
		return err
	}

	schedule := &api.Schedule{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: api.DefaultNamespace,
			Name:      o.BackupOptions.Name,
		},
		Spec: api.ScheduleSpec{
			Template: api.BackupSpec{
				IncludedNamespaces: o.BackupOptions.IncludeNamespaces,
				ExcludedNamespaces: o.BackupOptions.ExcludeNamespaces,
				IncludedResources:  o.BackupOptions.IncludeResources,
				ExcludedResources:  o.BackupOptions.ExcludeResources,
				LabelSelector:      o.BackupOptions.Selector.LabelSelector,
				SnapshotVolumes:    o.BackupOptions.SnapshotVolumes.Value,
				TTL:                metav1.Duration{Duration: o.BackupOptions.TTL},
			},
			Schedule: o.Schedule,
		},
	}

	if printed, err := output.PrintWithFormat(c, schedule); printed || err != nil {
		return err
	}

	_, err = arkClient.ArkV1().Schedules(schedule.Namespace).Create(schedule)
	if err != nil {
		return err
	}

	fmt.Printf("Schedule %q created successfully.\n", schedule.Name)
	return nil
}
