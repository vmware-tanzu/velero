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

package repository

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/label"
)

// A BackupRepositoryKey uniquely identify a backup repository
type BackupRepositoryKey struct {
	VolumeNamespace string
	BackupLocation  string
	RepositoryType  string
}

var (
	backupRepoNotFoundError = errors.New("backup repository not found")
)

func repoLabelsFromKey(key BackupRepositoryKey) labels.Set {
	return map[string]string{
		velerov1api.VolumeNamespaceLabel: label.GetValidName(key.VolumeNamespace),
		velerov1api.StorageLocationLabel: label.GetValidName(key.BackupLocation),
		velerov1api.RepositoryTypeLabel:  label.GetValidName(key.RepositoryType),
	}
}

// GetBackupRepository gets a backup repository through BackupRepositoryKey and ensure ready if required.
func GetBackupRepository(ctx context.Context, cli client.Client, namespace string, key BackupRepositoryKey, options ...bool) (*velerov1api.BackupRepository, error) {
	var ensureReady = true
	if len(options) > 0 {
		ensureReady = options[0]
	}

	selector := labels.SelectorFromSet(repoLabelsFromKey(key))

	backupRepoList := &velerov1api.BackupRepositoryList{}
	err := cli.List(ctx, backupRepoList, &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: selector,
	})

	if err != nil {
		return nil, errors.Wrap(err, "error getting backup repository list")
	}

	if len(backupRepoList.Items) == 0 {
		return nil, backupRepoNotFoundError
	}

	if len(backupRepoList.Items) > 1 {
		return nil, errors.Errorf("more than one BackupRepository found for workload namespace %q, backup storage location %q, repository type %q", key.VolumeNamespace, key.BackupLocation, key.RepositoryType)
	}

	repo := &backupRepoList.Items[0]

	if ensureReady {
		if repo.Status.Phase != velerov1api.BackupRepositoryPhaseReady {
			return nil, errors.Errorf("backup repository is not ready: %s", repo.Status.Message)
		}
	}

	return repo, nil
}
