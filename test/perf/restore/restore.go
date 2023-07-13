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

package restore

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"

	. "github.com/vmware-tanzu/velero/test"
	. "github.com/vmware-tanzu/velero/test/perf/test"
	. "github.com/vmware-tanzu/velero/test/util/velero"
)

type RestoreTest struct {
	TestCase
}

func (r *RestoreTest) Init() error {
	r.TestCase.Init()
	r.Ctx, r.CtxCancel = context.WithTimeout(context.Background(), 1*time.Hour)
	r.CaseBaseName = "restore"
	r.RestoreName = "restore-" + r.CaseBaseName + "-" + r.UUIDgen

	r.TestMsg = &TestMSG{
		Desc:      "Do restore resources for performance test",
		FailedMSG: "Failed to restore resources",
		Text:      fmt.Sprintf("Should restore resources success"),
	}
	return r.clearUpResourcesBeforRestore()
}

func (r *RestoreTest) clearUpResourcesBeforRestore() error {
	// we need to clear up all resources before do the restore test
	return r.TestCase.Destroy()
}

func (r *RestoreTest) Restore() error {
	// put partial parameters initialization here because we could not get latest backups in init periods for
	// velero may not ready
	backupName, err := GetLatestSuccessBackupsFromBSL(r.Ctx, VeleroCfg.VeleroCLI, "default")
	if err != nil {
		return errors.Wrapf(err, "failed to get backup to do the restore test")
	}

	r.BackupName = backupName
	r.RestoreArgs = []string{
		"create", "--namespace", VeleroCfg.VeleroNamespace, "restore", r.RestoreName,
		"--from-backup", r.BackupName, "--wait",
	}

	return r.TestCase.Restore()
}
func (r *RestoreTest) Destroy() error {
	return nil
}
