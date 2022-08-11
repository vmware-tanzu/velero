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

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

// Provider which is designed for one pod volumn to do the backup or restore
type Provider interface {
	// RunBackup which will do backup for one specific volumn and return snapshotID error
	// updateFunc which is used for update backup progress into related pvb status
	RunBackup(
		ctx context.Context,
		path string,
		tags map[string]string,
		parentSnapshot string,
		updateFunc func(velerov1api.PodVolumeOperationProgress)) (string, error)
	// RunRestore which will do restore for one specific volumn with given snapshot id and return error
	// updateFunc which is used for update restore progress into related pvr status
	RunRestore(
		ctx context.Context,
		snapshotID string,
		volumePath string,
		updateFunc func(velerov1api.PodVolumeOperationProgress)) error
	// Close which will close related repository
	Close(ctx context.Context)
}
