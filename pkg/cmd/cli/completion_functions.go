/*
Copyright The Velero Contributors.

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

package cli

import (
	"context"
	"strings"
	"time"

	"github.com/spf13/cobra"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/client"
)

// completionFunc is the function signature for cobra's ValidArgsFunction.
type completionFunc = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

// completeNames is a generic helper that builds a completion function for any
// Velero list type. The caller provides a constructor for the list object and a
// callback that extracts resource names from it.
func completeNames[L kbclient.ObjectList](f client.Factory, newList func() L, getNames func(L) []string) completionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		kbClient, err := f.KubebuilderClient()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		list := newList()
		if err := kbClient.List(ctx, list, &kbclient.ListOptions{Namespace: f.Namespace()}); err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var filtered []string
		for _, name := range getNames(list) {
			if strings.HasPrefix(name, toComplete) {
				filtered = append(filtered, name)
			}
		}
		return filtered, cobra.ShellCompDirectiveNoFileComp
	}
}

func CompleteBackupNames(f client.Factory) completionFunc {
	return completeNames(f,
		func() *velerov1api.BackupList { return new(velerov1api.BackupList) },
		func(l *velerov1api.BackupList) []string {
			names := make([]string, len(l.Items))
			for i, b := range l.Items {
				names[i] = b.Name
			}
			return names
		},
	)
}

func CompleteRestoreNames(f client.Factory) completionFunc {
	return completeNames(f,
		func() *velerov1api.RestoreList { return new(velerov1api.RestoreList) },
		func(l *velerov1api.RestoreList) []string {
			names := make([]string, len(l.Items))
			for i, r := range l.Items {
				names[i] = r.Name
			}
			return names
		},
	)
}

func CompleteScheduleNames(f client.Factory) completionFunc {
	return completeNames(f,
		func() *velerov1api.ScheduleList { return new(velerov1api.ScheduleList) },
		func(l *velerov1api.ScheduleList) []string {
			names := make([]string, len(l.Items))
			for i, s := range l.Items {
				names[i] = s.Name
			}
			return names
		},
	)
}

func CompleteBackupStorageLocationNames(f client.Factory) completionFunc {
	return completeNames(f,
		func() *velerov1api.BackupStorageLocationList { return new(velerov1api.BackupStorageLocationList) },
		func(l *velerov1api.BackupStorageLocationList) []string {
			names := make([]string, len(l.Items))
			for i, loc := range l.Items {
				names[i] = loc.Name
			}
			return names
		},
	)
}

func CompleteVolumeSnapshotLocationNames(f client.Factory) completionFunc {
	return completeNames(f,
		func() *velerov1api.VolumeSnapshotLocationList { return new(velerov1api.VolumeSnapshotLocationList) },
		func(l *velerov1api.VolumeSnapshotLocationList) []string {
			names := make([]string, len(l.Items))
			for i, loc := range l.Items {
				names[i] = loc.Name
			}
			return names
		},
	)
}

func CompleteBackupRepositoryNames(f client.Factory) completionFunc {
	return completeNames(f,
		func() *velerov1api.BackupRepositoryList { return new(velerov1api.BackupRepositoryList) },
		func(l *velerov1api.BackupRepositoryList) []string {
			names := make([]string, len(l.Items))
			for i, r := range l.Items {
				names[i] = r.Name
			}
			return names
		},
	)
}
