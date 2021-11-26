/*
Copyright 2021 the Velero contributors.

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

package filtering

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	. "github.com/vmware-tanzu/velero/test/e2e"
	. "github.com/vmware-tanzu/velero/test/e2e/test"
	. "github.com/vmware-tanzu/velero/test/e2e/util/k8s"
)

/*
Resources with the label velero.io/exclude-from-backup=true are not included
in backup, even if it contains a matching selector label.
*/

type ExcludeFromBackup struct {
	FilteringCase
}

var ExcludeFromBackupTest func() = TestFunc(&ExcludeFromBackup{testInBackup})

func (e *ExcludeFromBackup) Init() {
	rand.Seed(time.Now().UnixNano())
	UUIDgen, _ = uuid.NewRandom()
	e.FilteringCase.Init()
	e.BackupName = "backup-exclude-from-backup-" + UUIDgen.String()
	e.RestoreName = "restore-exclude-from-backup-" + UUIDgen.String()
	e.NSBaseName = "exclude-from-backup-" + UUIDgen.String()
	e.TestMsg = &TestMSG{
		Desc:      "Backup with the label velero.io/exclude-from-backup=true are not included test",
		Text:      "Should not backup resources with the label velero.io/exclude-from-backup=true",
		FailedMSG: "Failed to backup resources with the label velero.io/exclude-from-backup=true",
	}
	e.labels = map[string]string{
		"velero.io/exclude-from-backup": "true",
	}
	e.labelSelector = "velero.io/exclude-from-backup"
	e.BackupArgs = []string{
		"create", "--namespace", VeleroCfg.VeleroNamespace, "backup", e.BackupName,
		"--default-volumes-to-restic", "--wait",
	}

	e.RestoreArgs = []string{
		"create", "--namespace", VeleroCfg.VeleroNamespace, "restore", e.RestoreName,
		"--from-backup", e.BackupName, "--wait",
	}
}

func (e *ExcludeFromBackup) CreateResources() error {
	e.Ctx, _ = context.WithTimeout(context.Background(), 60*time.Minute)
	for nsNum := 0; nsNum < e.NamespacesTotal; nsNum++ {
		namespace := fmt.Sprintf("%s-%00000d", e.NSBaseName, nsNum)
		fmt.Printf("Creating resources in namespace ...%s\n", namespace)
		labels := e.labels
		if nsNum%2 == 0 {
			labels = map[string]string{
				"velero.io/exclude-from-backup": "false",
			}
		}
		if err := CreateNamespaceWithLabel(e.Ctx, e.Client, namespace, labels); err != nil {
			return errors.Wrapf(err, "Failed to create namespace %s", namespace)
		}

		//Create deployment
		fmt.Printf("Creating deployment in namespaces ...%s\n", namespace)

		deployment := NewDeployment(e.NSBaseName, namespace, e.replica, labels)
		deployment, err := CreateDeployment(e.Client.ClientGo, namespace, deployment)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to delete the namespace %q", namespace))
		}
		err = WaitForReadyDeployment(e.Client.ClientGo, namespace, deployment.Name)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to ensure deployment completion in namespace: %q", namespace))
		}
	}
	return nil
}

func (e *ExcludeFromBackup) Verify() error {
	for nsNum := 0; nsNum < e.NamespacesTotal; nsNum++ {
		namespace := fmt.Sprintf("%s-%00000d", e.NSBaseName, nsNum)
		fmt.Printf("Checking resources in namespaces ...%s\n", namespace)
		//Check deployment
		_, err := GetDeployment(e.Client.ClientGo, namespace, e.NSBaseName)
		if nsNum%2 == 0 { //include
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("failed to list deployment in namespace: %q", namespace))
			}
		} else { //exclude
			if err == nil {
				return fmt.Errorf("failed to exclude deployment in namespaces %q", namespace)
			} else {
				if apierrors.IsNotFound(err) { //resource should be excluded
					return nil
				}
				return errors.Wrap(err, fmt.Sprintf("failed to list deployment in namespace: %q", namespace))
			}
		}
	}
	return nil
}
