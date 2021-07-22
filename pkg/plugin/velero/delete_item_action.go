/*
Copyright 2020, 2021 the Velero contributors.

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
	"k8s.io/apimachinery/pkg/runtime"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

// DeleteItemActionExecuteInput contains the input parameters for the ItemAction's Execute function.
type DeleteItemActionExecuteInput struct {
	// Item is the item taken from the pristine backed up version of resource.
	Item runtime.Unstructured
	// Backup is the representation of the restore resource processed by Velero.
	Backup *velerov1api.Backup
}
