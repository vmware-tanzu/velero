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

package provider

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"

	"github.com/vmware-tanzu/velero/internal/credentials"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/uploader"
)

const restoreProgressCheckInterval = 10 * time.Second
const backupProgressCheckInterval = 10 * time.Second

// Provider which is designed for one pod volumn to do the backup or restore
type Provider interface {
	// RunBackup which will do backup for one specific volumn and return snapshotID error
	// updater which is used for update backup progress into related pvb status
	RunBackup(
		ctx context.Context,
		path string,
		tags map[string]string,
		parentSnapshot string,
		updater velerov1api.ProgressUpdater) (string, error)
	// RunRestore which will do restore for one specific volumn with given snapshot id and return error
	// updateFunc which is used for update restore progress into related pvr status
	RunRestore(
		ctx context.Context,
		snapshotID string,
		volumePath string,
		updater velerov1api.ProgressUpdater) error
	// Close which will close related repository
	Close(ctx context.Context)
}

//NewUploaderProvider initialize provider with specific uploader_type
func NewUploaderProvider(
	ctx context.Context,
	uploader_type string,
	repoIdentifier string,
	bsl *velerov1api.BackupStorageLocation,
	credGetter *credentials.CredentialGetter,
	repoKeySelector *v1.SecretKeySelector,
	log logrus.FieldLogger,
) (Provider, error) {
	if uploader_type == uploader.KopiaType {
		return NewResticUploaderProvider(repoIdentifier, bsl, credGetter, repoKeySelector, log)
	} else {
		return NewKopiaUploaderProvider(ctx, credGetter, bsl, log)
	}
}
