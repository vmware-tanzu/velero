/*
Copyright the Velero contributors.

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

package cacert

import (
	"context"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

// GetCACertFromBackup fetches the BackupStorageLocation for a backup and returns its cacert
func GetCACertFromBackup(ctx context.Context, client kbclient.Client, namespace string, backup *velerov1api.Backup) (string, error) {
	return GetCACertFromBSL(ctx, client, namespace, backup.Spec.StorageLocation)
}

// GetCACertFromRestore fetches the BackupStorageLocation for a restore's backup and returns its cacert
func GetCACertFromRestore(ctx context.Context, client kbclient.Client, namespace string, restore *velerov1api.Restore) (string, error) {
	// First get the backup that this restore references
	backup := &velerov1api.Backup{}
	key := kbclient.ObjectKey{
		Namespace: namespace,
		Name:      restore.Spec.BackupName,
	}

	if err := client.Get(ctx, key, backup); err != nil {
		if apierrors.IsNotFound(err) {
			// Backup not found is not a fatal error for cacert retrieval
			return "", nil
		}
		return "", errors.Wrapf(err, "error getting backup %s", restore.Spec.BackupName)
	}

	return GetCACertFromBackup(ctx, client, namespace, backup)
}

// GetCACertFromBSL fetches a BackupStorageLocation directly and returns its cacert
func GetCACertFromBSL(ctx context.Context, client kbclient.Client, namespace, bslName string) (string, error) {
	if bslName == "" {
		return "", nil
	}

	bsl := &velerov1api.BackupStorageLocation{}
	key := kbclient.ObjectKey{
		Namespace: namespace,
		Name:      bslName,
	}

	if err := client.Get(ctx, key, bsl); err != nil {
		if apierrors.IsNotFound(err) {
			// BSL not found is not a fatal error, just means no cacert
			return "", nil
		}
		return "", errors.Wrapf(err, "error getting backup storage location %s", bslName)
	}

	if bsl.Spec.ObjectStorage != nil && len(bsl.Spec.ObjectStorage.CACert) > 0 {
		return string(bsl.Spec.ObjectStorage.CACert), nil
	}

	return "", nil
}
