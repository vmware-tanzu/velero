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

package plugin

const (
	PVBackupPlugin                          = "velero.io/pv"
	PodBackupPlugin                         = "velero.io/pod"
	ServiceAccountBackupPlugin              = "velero.io/service-account"
	RemapCRDVersionBackupPlugin             = "velero.io/crd-remap-version"
	JobRestorePlugin                        = "velero.io/job"
	PodRestorePlugin                        = "velero.io/pod"
	ResticRestorePlugin                     = "velero.io/restic"
	InitRestoreHookPlugin                   = "velero.io/init-restore-hook"
	ServiceRestorePlugin                    = "velero.io/service"
	ServiceAccountRestorePlugin             = "velero.io/service-account"
	AddPVCFromPodRestorePlugin              = "velero.io/add-pvc-from-pod"
	AddPVFromPVCRestorePlugin               = "velero.io/add-pv-from-pvc"
	ChangeStorageClassRestorePlugin         = "velero.io/change-storage-class"
	RoleBindingItemPlugin                   = "velero.io/role-bindings"
	ClusterRoleBindingRestorePlugin         = "velero.io/cluster-role-bindings"
	CRDV1PreserveUnknownFieldsRestorePlugin = "velero.io/crd-preserve-fields"
	ChangePVCNodeSelectorPlugin             = "velero.io/change-pvc-node-selector"
	APIServiceRestorePlugin                 = "velero.io/apiservice"
)

var DefaultPlugins = []string{
	PVBackupPlugin,
	PodBackupPlugin,
	ServiceAccountBackupPlugin,
	RemapCRDVersionBackupPlugin,
	JobRestorePlugin,
	PodRestorePlugin,
	ResticRestorePlugin,
	InitRestoreHookPlugin,
	ServiceRestorePlugin,
	ServiceAccountRestorePlugin,
	AddPVCFromPodRestorePlugin,
	AddPVFromPVCRestorePlugin,
	ChangeStorageClassRestorePlugin,
	RoleBindingItemPlugin,
	ClusterRoleBindingRestorePlugin,
	CRDV1PreserveUnknownFieldsRestorePlugin,
	ChangePVCNodeSelectorPlugin,
	APIServiceRestorePlugin,
}
