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

package restorehelper

const (
	// WaitInitContainer is the name of the init container added
	// to workload pods to help with restores.
	// If Velero needs to further process the volume data after PVC is
	// provisioned, this init container is used to block Pod from running
	// until the volume data is ready
	WaitInitContainer = "restore-wait"

	// This is the name of the init container added by pre-v1.10 for the same
	// purpose with WaitInitContainer.
	// For compatibility, we need to check it when restoring backups created by
	// old releases. The pods backed up by old releases may contain this init container
	// since the init container is not deleted after pod is restored.
	WaitInitContainerLegacy = "restic-wait"
)
