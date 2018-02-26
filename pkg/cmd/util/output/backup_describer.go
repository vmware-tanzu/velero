/*
Copyright 2017 the Heptio Ark contributors.

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

package output

import (
	"fmt"
	"strings"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DescribeBackup(backup *v1.Backup) string {
	return Describe(func(d *Describer) {
		d.DescribeMetadata(backup.ObjectMeta)

		d.Println()
		DescribeBackupSpec(d, backup.Spec)

		d.Println()
		deleting := backup.DeletionTimestamp != nil && !backup.DeletionTimestamp.Time.IsZero()
		DescribeBackupStatus(d, backup.Status, deleting)
	})
}

func DescribeBackupSpec(d *Describer, spec v1.BackupSpec) {
	// TODO make a helper for this and use it in all the describers.
	d.Printf("Namespaces:\n")
	var s string
	if len(spec.IncludedNamespaces) == 0 {
		s = "*"
	} else {
		s = strings.Join(spec.IncludedNamespaces, ", ")
	}
	d.Printf("\tIncluded:\t%s\n", s)
	if len(spec.ExcludedNamespaces) == 0 {
		s = "<none>"
	} else {
		s = strings.Join(spec.ExcludedNamespaces, ", ")
	}
	d.Printf("\tExcluded:\t%s\n", s)

	d.Println()
	d.Printf("Resources:\n")
	if len(spec.IncludedResources) == 0 {
		s = "*"
	} else {
		s = strings.Join(spec.IncludedResources, ", ")
	}
	d.Printf("\tIncluded:\t%s\n", s)
	if len(spec.ExcludedResources) == 0 {
		s = "<none>"
	} else {
		s = strings.Join(spec.ExcludedResources, ", ")
	}
	d.Printf("\tExcluded:\t%s\n", s)

	d.Printf("\tCluster-scoped:\t%s\n", BoolPointerString(spec.IncludeClusterResources, "excluded", "included", "auto"))

	d.Println()
	s = "<none>"
	if spec.LabelSelector != nil {
		s = metav1.FormatLabelSelector(spec.LabelSelector)
	}
	d.Printf("Label selector:\t%s\n", s)

	d.Println()
	d.Printf("Snapshot PVs:\t%s\n", BoolPointerString(spec.SnapshotVolumes, "false", "true", "auto"))

	d.Println()
	d.Printf("TTL:\t%s\n", spec.TTL.Duration)

	d.Println()
	if len(spec.Hooks.Resources) == 0 {
		d.Printf("Hooks:\t<none>\n")
	} else {
		d.Printf("Hooks:\n")
		d.Printf("\tResources:\n")
		for _, backupResourceHookSpec := range spec.Hooks.Resources {
			d.Printf("\t\t%s:\n", backupResourceHookSpec.Name)
			d.Printf("\t\t\tNamespaces:\n")
			var s string
			if len(spec.IncludedNamespaces) == 0 {
				s = "*"
			} else {
				s = strings.Join(spec.IncludedNamespaces, ", ")
			}
			d.Printf("\t\t\t\tIncluded:\t%s\n", s)
			if len(spec.ExcludedNamespaces) == 0 {
				s = "<none>"
			} else {
				s = strings.Join(spec.ExcludedNamespaces, ", ")
			}
			d.Printf("\t\t\t\tExcluded:\t%s\n", s)

			d.Println()
			d.Printf("\t\t\tResources:\n")
			if len(spec.IncludedResources) == 0 {
				s = "*"
			} else {
				s = strings.Join(spec.IncludedResources, ", ")
			}
			d.Printf("\t\t\t\tIncluded:\t%s\n", s)
			if len(spec.ExcludedResources) == 0 {
				s = "<none>"
			} else {
				s = strings.Join(spec.ExcludedResources, ", ")
			}
			d.Printf("\t\t\t\tExcluded:\t%s\n", s)

			d.Println()
			s = "<none>"
			if backupResourceHookSpec.LabelSelector != nil {
				s = metav1.FormatLabelSelector(backupResourceHookSpec.LabelSelector)
			}
			d.Printf("\t\t\tLabel selector:\t%s\n", s)

			for _, hook := range backupResourceHookSpec.Hooks {
				if hook.Exec != nil {
					d.Println()
					d.Printf("\t\t\tExec Hook:\n")
					d.Printf("\t\t\t\tContainer:\t%s\n", hook.Exec.Container)
					d.Printf("\t\t\t\tCommand:\t%s\n", strings.Join(hook.Exec.Command, " "))
					d.Printf("\t\t\t\tOn Error:\t%s\n", hook.Exec.OnError)
					d.Printf("\t\t\t\tTimeout:\t%s\n", hook.Exec.Timeout.Duration)
				}
			}
		}
	}

}

func DescribeBackupStatus(d *Describer, status v1.BackupStatus, deleting bool) {
	phase := status.Phase
	if phase == "" {
		phase = v1.BackupPhaseNew
	}
	if deleting {
		phase = "Deleting"
	}
	d.Printf("Phase:\t%s\n", phase)

	d.Println()
	d.Printf("Backup Format Version:\t%d\n", status.Version)

	d.Println()
	d.Printf("Expiration:\t%s\n", status.Expiration.Time)

	d.Println()
	d.Printf("Validation errors:")
	if len(status.ValidationErrors) == 0 {
		d.Printf("\t<none>\n")
	} else {
		for _, ve := range status.ValidationErrors {
			d.Printf("\t%s\n", ve)
		}
	}

	d.Println()
	if len(status.VolumeBackups) == 0 {
		d.Printf("Persistent Volumes: <none included>\n")
	} else {
		d.Printf("Persistent Volumes:\n")
		for pvName, info := range status.VolumeBackups {
			d.Printf("\t%s:\n", pvName)
			d.Printf("\t\tSnapshot ID:\t%s\n", info.SnapshotID)
			d.Printf("\t\tType:\t%s\n", info.Type)
			d.Printf("\t\tAvailability Zone:\t%s\n", info.AvailabilityZone)
			iops := "<N/A>"
			if info.Iops != nil {
				iops = fmt.Sprintf("%d", *info.Iops)
			}
			d.Printf("\t\tIOPS:\t%s\n", iops)
		}
	}
}
