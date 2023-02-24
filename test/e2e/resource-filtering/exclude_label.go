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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/pkg/errors"

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

func (e *ExcludeFromBackup) Init() error {
	e.FilteringCase.Init()
	e.BackupName = "backup-exclude-from-backup-" + UUIDgen.String()
	e.RestoreName = "restore-" + UUIDgen.String()
	e.NSBaseName = "exclude-from-backup-" + UUIDgen.String()
	e.TestMsg = &TestMSG{
		Desc:      "Backup with the label velero.io/exclude-from-backup=true are not included test",
		Text:      "Should not backup resources with the label velero.io/exclude-from-backup=true",
		FailedMSG: "Failed to backup resources with the label velero.io/exclude-from-backup=true",
	}
	for nsNum := 0; nsNum < e.NamespacesTotal; nsNum++ {
		createNSName := fmt.Sprintf("%s-%00000d", e.NSBaseName, nsNum)
		*e.NSIncluded = append(*e.NSIncluded, createNSName)
	}
	e.labels = map[string]string{
		"velero.io/exclude-from-backup": "true",
	}
	e.labelSelector = "velero.io/exclude-from-backup"

	e.BackupArgs = []string{
		"create", "--namespace", VeleroCfg.VeleroNamespace, "backup", e.BackupName,
		"--include-namespaces", e.NSBaseName,
		"--default-volumes-to-fs-backup", "--wait",
	}

	e.RestoreArgs = []string{
		"create", "--namespace", VeleroCfg.VeleroNamespace, "restore", e.RestoreName,
		"--from-backup", e.BackupName, "--wait",
	}
	return nil
}

func (e *ExcludeFromBackup) CreateResources() error {
	e.Ctx, _ = context.WithTimeout(context.Background(), 60*time.Minute)
	namespace := e.NSBaseName
	// These 2 labels for resources to be included
	label1 := map[string]string{
		"meaningless-label-resource-to-include": "true",
	}
	label2 := map[string]string{
		"velero.io/exclude-from-backup": "false",
	}
	fmt.Printf("Creating resources in namespace ...%s\n", namespace)
	if err := CreateNamespace(e.Ctx, e.Client, namespace); err != nil {
		return errors.Wrapf(err, "Failed to create namespace %s", namespace)
	}
	serviceAccountName := "default"
	// wait until the service account is created before patch the image pull secret
	if err := WaitUntilServiceAccountCreated(e.Ctx, e.Client, namespace, serviceAccountName, 10*time.Minute); err != nil {
		return errors.Wrapf(err, "failed to wait the service account %q created under the namespace %q", serviceAccountName, namespace)
	}
	// add the image pull secret to avoid the image pull limit issue of Docker Hub
	if err := PatchServiceAccountWithImagePullSecret(e.Ctx, e.Client, namespace, serviceAccountName, VeleroCfg.RegistryCredentialFile); err != nil {
		return errors.Wrapf(err, "failed to patch the service account %q under the namespace %q", serviceAccountName, namespace)
	}
	//Create deployment: to be included
	fmt.Printf("Creating deployment in namespaces ...%s\n", namespace)
	deployment := NewDeployment(e.NSBaseName, namespace, e.replica, label2, nil)
	deployment, err := CreateDeployment(e.Client.ClientGo, namespace, deployment)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to delete the namespace %q", namespace))
	}
	err = WaitForReadyDeployment(e.Client.ClientGo, namespace, deployment.Name)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to ensure job completion in namespace: %q", namespace))
	}
	//Create Secret
	secretName := e.NSBaseName
	fmt.Printf("Creating secret %s in namespaces ...%s\n", secretName, namespace)
	_, err = CreateSecret(e.Client.ClientGo, namespace, secretName, e.labels)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to create secret in the namespace %q", namespace))
	}
	err = WaitForSecretsComplete(e.Client.ClientGo, namespace, secretName)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to ensure secret completion in namespace: %q", namespace))
	}
	By(fmt.Sprintf("Checking secret %s should exists in namespaces ...%s\n", secretName, namespace), func() {
		_, err = GetSecret(e.Client.ClientGo, namespace, e.NSBaseName)
		Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("failed to list deployment in namespace: %q", namespace))
	})
	//Create Configmap: to be included
	configmaptName := e.NSBaseName
	fmt.Printf("Creating configmap %s in namespaces ...%s\n", configmaptName, namespace)
	_, err = CreateConfigMap(e.Client.ClientGo, namespace, configmaptName, label1, nil)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to create configmap in the namespace %q", namespace))
	}
	err = WaitForConfigMapComplete(e.Client.ClientGo, namespace, configmaptName)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to ensure secret completion in namespace: %q", namespace))
	}
	return nil
}

func (e *ExcludeFromBackup) Verify() error {
	namespace := e.NSBaseName
	By(fmt.Sprintf("Checking resources in namespaces ...%s\n", namespace), func() {
		//Check namespace
		checkNS, err := GetNamespace(e.Ctx, e.Client, namespace)
		Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("Could not retrieve test namespace %s", namespace))
		Expect(checkNS.Name == namespace).To(Equal(true), fmt.Sprintf("Retrieved namespace for %s has name %s instead", namespace, checkNS.Name))

		//Check deployment: should be included
		_, err = GetDeployment(e.Client.ClientGo, namespace, e.NSBaseName)
		Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("failed to list deployment in namespace: %q", namespace))

		//Check secrets: secrets should not be included
		_, err = GetSecret(e.Client.ClientGo, namespace, e.NSBaseName)
		Expect(err).Should(HaveOccurred(), fmt.Sprintf("failed to list deployment in namespace: %q", namespace))
		Expect(apierrors.IsNotFound(err)).To(Equal(true))

		//Check configmap: should be included
		_, err = GetConfigmap(e.Client.ClientGo, namespace, e.NSBaseName)
		Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("failed to list configmap in namespace: %q", namespace))
	})
	return nil
}
