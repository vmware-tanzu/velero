package schedule

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

//the ordered resources test related to https://github.com/vmware-tanzu/velero/issues/4561
import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	waitutil "k8s.io/apimachinery/pkg/util/wait"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	. "github.com/vmware-tanzu/velero/test/e2e/test"
	. "github.com/vmware-tanzu/velero/test/util/k8s"
	. "github.com/vmware-tanzu/velero/test/util/velero"
)

var ScheduleOrderedResources func() = TestFunc(&OrderedResources{})

type OrderedResources struct {
	Namespace    string
	ScheduleName string
	OrderMap     map[string]string
	ScheduleArgs []string
	TestCase
}

func (o *OrderedResources) Init() error {
	o.TestCase.Init()
	o.CaseBaseName = "ordered-resources-" + o.UUIDgen
	o.ScheduleName = "schedule-" + o.CaseBaseName
	o.Namespace = o.CaseBaseName + "-" + o.UUIDgen
	o.OrderMap = map[string]string{
		"deployments": fmt.Sprintf("deploy-%s", o.CaseBaseName),
		"secrets":     fmt.Sprintf("secret-%s", o.CaseBaseName),
		"configmaps":  fmt.Sprintf("configmap-%s", o.CaseBaseName),
	}
	o.TestMsg = &TestMSG{
		Desc:      "Create a schedule to backup resources in a specific order should be successful",
		FailedMSG: "Failed to verify schedule backup resources in a specific order",
		Text:      "Create a schedule to backup resources in a specific order should be successful",
	}
	o.ScheduleArgs = []string{"--schedule", "@every 1m",
		"--include-namespaces", o.Namespace, "--default-volumes-to-fs-backup", "--ordered-resources"}
	var orderStr string
	for kind, resource := range o.OrderMap {
		orderStr += fmt.Sprintf("%s=%s;", kind, resource)
	}
	o.ScheduleArgs = append(o.ScheduleArgs, strings.TrimRight(orderStr, ";"))

	return nil
}
func (o *OrderedResources) CreateResources() error {
	label := map[string]string{
		"orderedresources": "true",
	}
	fmt.Printf("Creating resources in %s namespace ...\n", o.Namespace)
	if err := CreateNamespace(o.Ctx, o.Client, o.Namespace); err != nil {
		return errors.Wrapf(err, "failed to create namespace %s", o.Namespace)
	}
	//Create deployment
	deploymentName := fmt.Sprintf("deploy-%s", o.CaseBaseName)
	fmt.Printf("Creating deployment %s in %s namespaces ...\n", deploymentName, o.Namespace)
	deployment := NewDeployment(deploymentName, o.Namespace, 1, label, nil).Result()
	deployment, err := CreateDeployment(o.Client.ClientGo, o.Namespace, deployment)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to create namespace %q with err %v", o.Namespace, err))
	}
	err = WaitForReadyDeployment(o.Client.ClientGo, o.Namespace, deployment.Name)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to ensure job completion in namespace: %q", o.Namespace))
	}
	//Create Secret
	secretName := fmt.Sprintf("secret-%s", o.CaseBaseName)
	fmt.Printf("Creating secret %s in %s namespaces ...\n", secretName, o.Namespace)
	_, err = CreateSecret(o.Client.ClientGo, o.Namespace, secretName, label)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to create secret in the namespace %q", o.Namespace))
	}
	err = WaitForSecretsComplete(o.Client.ClientGo, o.Namespace, secretName)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to ensure secret completion in namespace: %q", o.Namespace))
	}
	//Create Configmap
	configmapName := fmt.Sprintf("configmap-%s", o.CaseBaseName)
	fmt.Printf("Creating configmap %s in %s namespaces ...\n", configmapName, o.Namespace)
	_, err = CreateConfigMap(o.Client.ClientGo, o.Namespace, configmapName, label, nil)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to create configmap in the namespace %q", o.Namespace))
	}
	err = WaitForConfigMapComplete(o.Client.ClientGo, o.Namespace, configmapName)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to ensure secret completion in namespace: %q", o.Namespace))
	}
	return nil
}

func (o *OrderedResources) Backup() error {
	By(fmt.Sprintf("Create schedule the workload in %s namespace", o.Namespace), func() {
		err := VeleroScheduleCreate(o.Ctx, o.VeleroCfg.VeleroCLI, o.VeleroCfg.VeleroNamespace, o.ScheduleName, o.ScheduleArgs)
		Expect(err).To(Succeed(), fmt.Sprintf("Failed to create schedule %s  with err %v", o.ScheduleName, err))
	})
	return nil
}

func (o *OrderedResources) Destroy() error {
	return nil
}

func (o *OrderedResources) Verify() error {
	By(fmt.Sprintf("Checking resource order in %s schedule cr", o.ScheduleName), func() {
		err := CheckScheduleWithResourceOrder(o.Ctx, o.VeleroCfg.VeleroCLI, o.VeleroCfg.VeleroNamespace, o.ScheduleName, o.OrderMap)
		Expect(err).To(Succeed(), fmt.Sprintf("Failed to check schedule %s with err %v", o.ScheduleName, err))
	})

	By("Checking resource order in backup cr", func() {
		backupList := new(velerov1api.BackupList)
		err := waitutil.PollImmediate(10*time.Second, time.Minute*5, func() (bool, error) {
			if err := o.Client.Kubebuilder.List(o.Ctx, backupList, &kbclient.ListOptions{Namespace: o.VeleroCfg.VeleroNamespace}); err != nil {
				return false, fmt.Errorf("failed to list backup object in %s namespace with err %v", o.VeleroCfg.VeleroNamespace, err)
			}

			for _, backup := range backupList.Items {
				if err := CheckBackupWithResourceOrder(o.Ctx, o.VeleroCfg.VeleroCLI, o.VeleroCfg.VeleroNamespace, backup.Name, o.OrderMap); err == nil {
					return true, nil
				}
			}
			fmt.Printf("still finding backup created by schedule %s ...\n", o.ScheduleName)
			return false, nil
		})
		Expect(err).To(Succeed(), fmt.Sprintf("Failed to check schedule %s created backup with err %v", o.ScheduleName, err))
	})
	return nil
}

func (o *OrderedResources) Clean() error {
	if CurrentSpecReport().Failed() && o.VeleroCfg.FailFast {
		fmt.Println("Test case failed and fail fast is enabled. Skip resource clean up.")
	} else {
		Expect(VeleroScheduleDelete(o.Ctx, o.VeleroCfg.VeleroCLI, o.VeleroCfg.VeleroNamespace, o.ScheduleName)).To(Succeed())
		Expect(o.TestCase.Clean()).To(Succeed())
	}

	return nil
}

func (o *OrderedResources) DeleteAllBackups() error {
	backupList := new(velerov1api.BackupList)
	if err := o.Client.Kubebuilder.List(o.Ctx, backupList, &kbclient.ListOptions{Namespace: o.VeleroCfg.VeleroNamespace}); err != nil {
		return fmt.Errorf("failed to list backup object in %s namespace with err %v", o.VeleroCfg.VeleroNamespace, err)
	}
	for _, backup := range backupList.Items {
		if err := VeleroBackupDelete(o.Ctx, o.VeleroCfg.VeleroCLI, o.VeleroCfg.VeleroNamespace, backup.Name); err != nil {
			return err
		}
	}
	return nil
}
