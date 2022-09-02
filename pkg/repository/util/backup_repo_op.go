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

package util

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

// GetBackupRepositoryByLabel which find backup repository through pvbNamespace, label
// name of BackupRepository is generated with prefix volumeNamespace-backupLocation- and end with random characters
// it could not retrieve the BackupRepository CR with namespace + name. so first list all CRs with in the pvbNamespace
// then filtering the matched CR by label
func GetBackupRepositoryByLabel(ctx context.Context, cli client.Client, pvbNamespace string, selector labels.Selector) (velerov1api.BackupRepository, error) {
	backupRepoList := &velerov1api.BackupRepositoryList{}
	if err := cli.List(ctx, backupRepoList, &client.ListOptions{
		Namespace:     pvbNamespace,
		LabelSelector: selector,
	}); err != nil {
		return velerov1api.BackupRepository{}, errors.Wrap(err, "error getting backup repository list")
	} else if len(backupRepoList.Items) == 1 {
		return backupRepoList.Items[0], nil
	} else {
		return velerov1api.BackupRepository{}, errors.Errorf("unexpectedly find %d BackupRepository for workload namespace %s with label selector %v", len(backupRepoList.Items), pvbNamespace, selector)
	}
}
