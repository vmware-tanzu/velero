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

package features

import (
	"errors"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework/common"
)

type PluginFinder interface {
	Find(kind common.PluginKind, name string) bool
}

type Verifier struct {
	finder PluginFinder
}

func NewVerifier(finder PluginFinder) *Verifier {
	return &Verifier{
		finder: finder,
	}
}

func (v *Verifier) Verify(name string) (bool, error) {
	enabled := IsEnabled(name)

	switch name {
	case velerov1api.CSIFeatureFlag:
		return verifyCSIFeature(v.finder, enabled)
	default:
		return false, nil
	}
}

func verifyCSIFeature(finder PluginFinder, enabled bool) (bool, error) {
	installed := false
	installed = finder.Find(common.PluginKindBackupItemActionV2, "velero.io/csi-pvc-backupper")
	if installed {
		installed = finder.Find(common.PluginKindRestoreItemActionV2, "velero.io/csi-pvc-restorer")
	}

	if !enabled && installed {
		return false, errors.New("CSI plugins are registered, but the EnableCSI feature is not enabled")
	} else if enabled && !installed {
		return false, errors.New("CSI feature is enabled, but CSI plugins are not registered")
	} else if !enabled && !installed {
		return false, nil
	} else {
		return true, nil
	}
}
