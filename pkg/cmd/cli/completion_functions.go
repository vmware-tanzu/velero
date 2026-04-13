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

	"github.com/spf13/cobra"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/client"
)

// completionFunc is the function signature for cobra's ValidArgsFunction.
type completionFunc = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

// CompleteBackupNames returns a completion function that lists backup names from the cluster.
func CompleteBackupNames(f client.Factory) completionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		kbClient, err := f.KubebuilderClient()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		backups := new(velerov1api.BackupList)
		if err := kbClient.List(context.TODO(), backups, &kbclient.ListOptions{Namespace: f.Namespace()}); err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var names []string
		for _, b := range backups.Items {
			if strings.HasPrefix(b.Name, toComplete) {
				names = append(names, b.Name)
			}
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	}
}

// CompleteRestoreNames returns a completion function that lists restore names from the cluster.
func CompleteRestoreNames(f client.Factory) completionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		kbClient, err := f.KubebuilderClient()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		restores := new(velerov1api.RestoreList)
		if err := kbClient.List(context.TODO(), restores, &kbclient.ListOptions{Namespace: f.Namespace()}); err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var names []string
		for _, r := range restores.Items {
			if strings.HasPrefix(r.Name, toComplete) {
				names = append(names, r.Name)
			}
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	}
}

// CompleteScheduleNames returns a completion function that lists schedule names from the cluster.
func CompleteScheduleNames(f client.Factory) completionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		kbClient, err := f.KubebuilderClient()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		schedules := new(velerov1api.ScheduleList)
		if err := kbClient.List(context.TODO(), schedules, &kbclient.ListOptions{Namespace: f.Namespace()}); err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var names []string
		for _, s := range schedules.Items {
			if strings.HasPrefix(s.Name, toComplete) {
				names = append(names, s.Name)
			}
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	}
}

// CompleteBackupStorageLocationNames returns a completion function that lists backup storage location names from the cluster.
func CompleteBackupStorageLocationNames(f client.Factory) completionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		kbClient, err := f.KubebuilderClient()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		locations := new(velerov1api.BackupStorageLocationList)
		if err := kbClient.List(context.TODO(), locations, &kbclient.ListOptions{Namespace: f.Namespace()}); err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var names []string
		for _, l := range locations.Items {
			if strings.HasPrefix(l.Name, toComplete) {
				names = append(names, l.Name)
			}
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	}
}

// CompleteVolumeSnapshotLocationNames returns a completion function that lists volume snapshot location names from the cluster.
func CompleteVolumeSnapshotLocationNames(f client.Factory) completionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		kbClient, err := f.KubebuilderClient()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		locations := new(velerov1api.VolumeSnapshotLocationList)
		if err := kbClient.List(context.TODO(), locations, &kbclient.ListOptions{Namespace: f.Namespace()}); err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var names []string
		for _, l := range locations.Items {
			if strings.HasPrefix(l.Name, toComplete) {
				names = append(names, l.Name)
			}
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	}
}

// CompleteBackupRepositoryNames returns a completion function that lists backup repository names from the cluster.
func CompleteBackupRepositoryNames(f client.Factory) completionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		kbClient, err := f.KubebuilderClient()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		repos := new(velerov1api.BackupRepositoryList)
		if err := kbClient.List(context.TODO(), repos, &kbclient.ListOptions{Namespace: f.Namespace()}); err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var names []string
		for _, r := range repos.Items {
			if strings.HasPrefix(r.Name, toComplete) {
				names = append(names, r.Name)
			}
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	}
}
