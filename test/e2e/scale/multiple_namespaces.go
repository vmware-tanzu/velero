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

package scale

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/google/uuid"

	. "github.com/vmware-tanzu/velero/test/e2e/util/k8s"
	. "github.com/vmware-tanzu/velero/test/e2e/util/velero"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/vmware-tanzu/velero/test/e2e"
)

func BasicBackupRestore() {

	client, err := NewTestClient()
	Expect(err).To(Succeed(), "Failed to instantiate cluster client for multiple namespace tests")

	BeforeEach(func() {
		var err error
		flag.Parse()
		UUIDgen, err = uuid.NewRandom()
		Expect(err).To(Succeed())
		if VeleroCfg.InstallVelero {
			Expect(VeleroInstall(context.Background(), &VeleroCfg, "", false)).To(Succeed())
		}
	})

	AfterEach(func() {
		if VeleroCfg.InstallVelero {
			err := VeleroUninstall(context.Background(), VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace)
			Expect(err).To(Succeed())
		}

	})

	Context("When I create 2 namespaces", func() {
		It("should be successfully backed up and restored", func() {
			backupName := "backup-" + UUIDgen.String()
			restoreName := "restore-" + UUIDgen.String()
			fiveMinTimeout, _ := context.WithTimeout(context.Background(), 5*time.Minute)
			Expect(RunMultipleNamespaceTest(fiveMinTimeout, client, "nstest-"+UUIDgen.String(), 2,
				backupName, restoreName)).To(Succeed(), "Failed to successfully backup and restore multiple namespaces")
		})
	})
}

func MultiNSBackupRestore() {

	client, err := NewTestClient()
	Expect(err).To(Succeed(), "Failed to instantiate cluster client for multiple namespace tests")

	BeforeEach(func() {
		var err error
		flag.Parse()
		UUIDgen, err = uuid.NewRandom()
		Expect(err).To(Succeed())
		if VeleroCfg.InstallVelero {
			Expect(VeleroInstall(context.Background(), &VeleroCfg, "", false)).To(Succeed())
		}
	})

	AfterEach(func() {
		if VeleroCfg.InstallVelero {
			err := VeleroUninstall(context.Background(), VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace)
			Expect(err).To(Succeed())
		}

	})

	Context("When I create 2500 namespaces", func() {
		It("should be successfully backed up and restored", func() {
			backupName := "backup-" + UUIDgen.String()
			restoreName := "restore-" + UUIDgen.String()
			twoHourTimeout, _ := context.WithTimeout(context.Background(), 2*time.Hour)
			Expect(RunMultipleNamespaceTest(twoHourTimeout, client, "nstest-"+UUIDgen.String(), 2500,
				backupName, restoreName)).To(Succeed(), "Failed to successfully backup and restore multiple namespaces")
		})
	})
}

func RunMultipleNamespaceTest(ctx context.Context, client TestClient, nsBaseName string, numberOfNamespaces int, backupName string, restoreName string) error {
	defer CleanupNamespaces(context.Background(), client, nsBaseName) // Run at exit for final cleanup
	var excludeNamespaces []string

	// Currently it's hard to build a large list of namespaces to include and wildcards do not work so instead
	// we will exclude all of the namespaces that existed prior to the test from the backup
	namespaces, err := client.ClientGo.CoreV1().Namespaces().List(ctx, v1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "Could not retrieve namespaces")
	}

	for _, excludeNamespace := range namespaces.Items {
		excludeNamespaces = append(excludeNamespaces, excludeNamespace.Name)
	}

	fmt.Printf("Creating namespaces ...\n")
	for nsNum := 0; nsNum < numberOfNamespaces; nsNum++ {
		createNSName := fmt.Sprintf("%s-%00000d", nsBaseName, nsNum)
		if err := CreateNamespace(ctx, client, createNSName); err != nil {
			return errors.Wrapf(err, "Failed to create namespace %s", createNSName)
		}
	}
	if err := VeleroBackupExcludeNamespaces(ctx, VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace, backupName, excludeNamespaces); err != nil {
		RunDebug(context.Background(), VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace, backupName, "")
		return errors.Wrapf(err, "Failed to backup backup namespaces %s-*", nsBaseName)
	}

	err = CleanupNamespaces(ctx, client, nsBaseName)
	if err != nil {
		return errors.Wrap(err, "Could cleanup retrieve namespaces")
	}

	err = VeleroRestore(ctx, VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace, restoreName, backupName)
	if err != nil {
		RunDebug(context.Background(), VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace, "", restoreName)
		return errors.Wrap(err, "Restore failed")
	}

	// Verify that we got back all of the namespaces we created
	for nsNum := 0; nsNum < numberOfNamespaces; nsNum++ {
		checkNSName := fmt.Sprintf("%s-%00000d", nsBaseName, nsNum)
		checkNS, err := GetNamespace(ctx, client, checkNSName)
		if err != nil {
			return errors.Wrapf(err, "Could not retrieve test namespace %s", checkNSName)
		}
		if checkNS.Name != checkNSName {
			return errors.Errorf("Retrieved namespace for %s has name %s instead", checkNSName, checkNS.Name)
		}
	}
	// Cleanup is automatic on the way out
	return nil
}
