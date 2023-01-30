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
	"context"
	"flag"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	waitutil "k8s.io/apimachinery/pkg/util/wait"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	. "github.com/vmware-tanzu/velero/test/e2e"
	. "github.com/vmware-tanzu/velero/test/e2e/test"
	. "github.com/vmware-tanzu/velero/test/e2e/util/k8s"
	. "github.com/vmware-tanzu/velero/test/e2e/util/velero"
)

type OrderedResources struct {
	Namespace    string
	ScheduleName string
	OrderMap     map[string]string
	ScheduleArgs []string
	TestCase
}

func ScheduleOrderedResources() {
	veleroCfg := VeleroCfg
	BeforeEach(func() {
		flag.Parse()
		if veleroCfg.InstallVelero {
			veleroCfg.UseVolumeSnapshots = false
			Expect(VeleroInstall(context.Background(), &veleroCfg)).To(Succeed())
		}
	})

	AfterEach(func() {
		if veleroCfg.InstallVelero && !veleroCfg.Debug {
			Expect(VeleroUninstall(context.Background(), veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace)).To(Succeed())
		}
	})

	It("Create a schedule to backup resources in a specific order should be successful", func() {
		test := &OrderedResources{}
		test.VeleroCfg = VeleroCfg
		err := test.Init()
		Expect(err).To(Succeed(), err)
		defer func() {
			Expect(DeleteNamespace(test.Ctx, test.Client, test.Namespace, false)).To(Succeed(), fmt.Sprintf("Failed to delete the namespace %s", test.Namespace))
			err = VeleroScheduleDelete(test.Ctx, veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace, test.ScheduleName)
			Expect(err).To(Succeed(), fmt.Sprintf("Failed to delete schedule with err %v", err))
			err = test.DeleteBackups()
			Expect(err).To(Succeed(), fmt.Sprintf("Failed to delete backups with err %v", err))
		}()

		By(fmt.Sprintf("Prepare workload as target to backup in base namespace %s", test.Namespace), func() {
			err = test.CreateResources()
			Expect(err).To(Succeed(), fmt.Sprintf("Failed to create resources to backup with err %v", err))
		})

		By(fmt.Sprintf("Create schedule the workload in %s namespace", test.Namespace), func() {
			err = VeleroScheduleCreate(test.Ctx, veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace, test.ScheduleName, test.ScheduleArgs)
			Expect(err).To(Succeed(), fmt.Sprintf("Failed to create schedule %s  with err %v", test.ScheduleName, err))
		})

		By(fmt.Sprintf("Checking resource order in %s schedule cr", test.ScheduleName), func() {
			err = CheckScheduleWithResourceOrder(test.Ctx, veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace, test.ScheduleName, test.OrderMap)
			Expect(err).To(Succeed(), fmt.Sprintf("Failed to check schedule %s with err %v", test.ScheduleName, err))
		})

		By("Checking resource order in backup cr", func() {
			backupList := new(velerov1api.BackupList)
			err = waitutil.PollImmediate(10*time.Second, time.Minute*5, func() (bool, error) {
				if err = test.Client.Kubebuilder.List(test.Ctx, backupList, &kbclient.ListOptions{Namespace: veleroCfg.VeleroNamespace}); err != nil {
					return false, fmt.Errorf("failed to list backup object in %s namespace with err %v", veleroCfg.VeleroNamespace, err)
				}

				for _, backup := range backupList.Items {
					if err = CheckBackupWithResourceOrder(test.Ctx, veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace, backup.Name, test.OrderMap); err == nil {
						return true, nil
					}
				}
				fmt.Printf("still finding backup created by schedule %s ...\n", test.ScheduleName)
				return false, nil
			})
			Expect(err).To(Succeed(), fmt.Sprintf("Failed to check schedule %s created backup with err %v", test.ScheduleName, err))
		})

	})
}

func (o *OrderedResources) Init() error {
	rand.Seed(time.Now().UnixNano())
	UUIDgen, _ = uuid.NewRandom()
	o.VeleroCfg = VeleroCfg
	o.Client = *o.VeleroCfg.ClientToInstallVelero
	o.ScheduleName = "schedule-ordered-resources-" + UUIDgen.String()
	o.NSBaseName = "schedule-ordered-resources"
	o.Namespace = o.NSBaseName + "-" + UUIDgen.String()
	o.OrderMap = map[string]string{
		"deployments": fmt.Sprintf("deploy-%s", o.NSBaseName),
		"secrets":     fmt.Sprintf("secret-%s", o.NSBaseName),
		"configmaps":  fmt.Sprintf("configmap-%s", o.NSBaseName),
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
	veleroCfg := o.VeleroCfg
	o.Ctx, _ = context.WithTimeout(context.Background(), 5*time.Minute)
	label := map[string]string{
		"orderedresources": "true",
	}
	fmt.Printf("Creating resources in %s namespace ...\n", o.Namespace)
	if err := CreateNamespace(o.Ctx, o.Client, o.Namespace); err != nil {
		return errors.Wrapf(err, "failed to create namespace %s", o.Namespace)
	}
	serviceAccountName := "default"
	// wait until the service account is created before patch the image pull secret
	if err := WaitUntilServiceAccountCreated(o.Ctx, o.Client, o.Namespace, serviceAccountName, 10*time.Minute); err != nil {
		return errors.Wrapf(err, "failed to wait the service account %q created under the namespace %q", serviceAccountName, o.Namespace)
	}
	// add the image pull secret to avoid the image pull limit issue of Docker Hub
	if err := PatchServiceAccountWithImagePullSecret(o.Ctx, o.Client, o.Namespace, serviceAccountName, veleroCfg.RegistryCredentialFile); err != nil {
		return errors.Wrapf(err, "failed to patch the service account %q under the namespace %q", serviceAccountName, o.Namespace)
	}
	//Create deployment
	deploymentName := fmt.Sprintf("deploy-%s", o.NSBaseName)
	fmt.Printf("Creating deployment %s in %s namespaces ...\n", deploymentName, o.Namespace)
	deployment := NewDeployment(deploymentName, o.Namespace, 1, label, nil)
	deployment, err := CreateDeployment(o.Client.ClientGo, o.Namespace, deployment)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to create namespace %q with err %v", o.Namespace, err))
	}
	err = WaitForReadyDeployment(o.Client.ClientGo, o.Namespace, deployment.Name)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to ensure job completion in namespace: %q", o.Namespace))
	}
	//Create Secret
	secretName := fmt.Sprintf("secret-%s", o.NSBaseName)
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
	configmapName := fmt.Sprintf("configmap-%s", o.NSBaseName)
	fmt.Printf("Creating configmap %s in %s namespaces ...\n", configmapName, o.Namespace)
	_, err = CreateConfigMap(o.Client.ClientGo, o.Namespace, configmapName, label)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to create configmap in the namespace %q", o.Namespace))
	}
	err = WaitForConfigMapComplete(o.Client.ClientGo, o.Namespace, configmapName)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to ensure secret completion in namespace: %q", o.Namespace))
	}
	return nil
}

func (o *OrderedResources) DeleteBackups() error {
	veleroCfg := o.VeleroCfg
	backupList := new(velerov1api.BackupList)
	if err := o.Client.Kubebuilder.List(o.Ctx, backupList, &kbclient.ListOptions{Namespace: veleroCfg.VeleroNamespace}); err != nil {
		return fmt.Errorf("failed to list backup object in %s namespace with err %v", veleroCfg.VeleroNamespace, err)
	}
	for _, backup := range backupList.Items {
		if err := VeleroBackupDelete(o.Ctx, veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace, backup.Name); err != nil {
			return err
		}
	}
	return nil
}
