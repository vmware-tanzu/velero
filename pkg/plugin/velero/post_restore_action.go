/*
Copyright 2020 the Velero contributors.

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

package velero

import (
	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

// PostRestoreAction provides a hook into the restore process after it completes.
type PostRestoreAction interface {
	// Execute the PostRestoreAction plugin providing it access to the Restore that
	// has been completed
	Execute(restore *api.Restore) error
}
