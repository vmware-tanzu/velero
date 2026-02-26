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

package filtering

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	. "github.com/vmware-tanzu/velero/test/e2e/test"
	. "github.com/vmware-tanzu/velero/test/util/k8s"
)

// WildcardNamespaces tests the inclusion and exclusion of namespaces using wildcards
// introduced in PR #9255 (Issue #1874).
type WildcardNamespaces struct {
	TestCase
	namespacesToInclude []string
	namespacesToExclude []string
}

var BackupWithWildcardNamespaces func() = TestFunc(&WildcardNamespaces{})

func (w *WildcardNamespaces) Init() error {
	Expect(w.TestCase.Init()).To(Succeed())

	// Use CaseBaseName as a prefix for all namespaces so that the base
	// TestCase.Destroy() and TestCase.Clean() can automatically garbage-collect them.
	w.CaseBaseName = "wildcard-ns-" + w.UUIDgen
	w.BackupName = "backup-" + w.CaseBaseName
	w.RestoreName = "restore-" + w.CaseBaseName

	// Define specific namespaces to test both wildcard include and exclude
	w.namespacesToInclude = []string{
		w.CaseBaseName + "-inc-wild-1", // Will be matched by *-inc-wild-*
		w.CaseBaseName + "-inc-wild-2", // Will be matched by *-inc-wild-*
		w.CaseBaseName + "-exact-inc",  // Exact match
	}
	w.namespacesToExclude = []string{
		w.CaseBaseName + "-exc-wild-1", // Excluded by *-exc-wild-*
		w.CaseBaseName + "-exc-wild-2", // Excluded by *-exc-wild-*
	}

	w.TestMsg = &TestMSG{
		Desc:      "Backup and restore with wildcard namespaces",
		Text:      "Should correctly backup and restore namespaces matching the include wildcard, and skip the exclude wildcard",
		FailedMSG: "Failed to properly filter namespaces using wildcards",
	}

	// Construct wildcard strings
	incWildcard := fmt.Sprintf("%s-inc-wild-*", w.CaseBaseName)
	excWildcard := fmt.Sprintf("%s-exc-wild-*", w.CaseBaseName)
	exactInc := fmt.Sprintf("%s-exact-inc", w.CaseBaseName)

	// In backup args, we intentionally include the `excWildcard` in the include-namespaces
	// to verify that `exclude-namespaces` correctly overrides it.
	w.BackupArgs = []string{
		"create", "--namespace", w.VeleroCfg.VeleroNamespace, "backup", w.BackupName,
		"--include-namespaces", fmt.Sprintf("%s,%s,%s", incWildcard, exactInc, excWildcard),
		"--exclude-namespaces", excWildcard,
		"--default-volumes-to-fs-backup", "--wait",
	}

	w.RestoreArgs = []string{
		"create", "--namespace", w.VeleroCfg.VeleroNamespace, "restore", w.RestoreName,
		"--from-backup", w.BackupName, "--wait",
	}

	return nil
}

func (w *WildcardNamespaces) CreateResources() error {
	allNamespaces := append(w.namespacesToInclude, w.namespacesToExclude...)

	for _, ns := range allNamespaces {
		By(fmt.Sprintf("Creating namespace %s", ns), func() {
			Expect(CreateNamespace(w.Ctx, w.Client, ns)).To(Succeed(), fmt.Sprintf("Failed to create namespace %s", ns))
		})

		// Create a ConfigMap in each namespace to verify resource restoration
		cmName := "configmap-" + ns
		By(fmt.Sprintf("Creating ConfigMap %s in namespace %s", cmName, ns), func() {
			_, err := CreateConfigMap(w.Client.ClientGo, ns, cmName, map[string]string{"wildcard-test": "true"}, nil)
			Expect(err).To(Succeed(), fmt.Sprintf("Failed to create configmap in namespace %s", ns))
		})
	}
	return nil
}

func (w *WildcardNamespaces) Verify() error {
	// 1. Verify included namespaces and their resources were fully restored
	for _, ns := range w.namespacesToInclude {
		By(fmt.Sprintf("Checking included namespace %s exists", ns), func() {
			_, err := GetNamespace(w.Ctx, w.Client, ns)
			Expect(err).To(Succeed(), fmt.Sprintf("Included namespace %s should exist after restore", ns))

			_, err = GetConfigMap(w.Client.ClientGo, ns, "configmap-"+ns)
			Expect(err).To(Succeed(), fmt.Sprintf("ConfigMap in included namespace %s should exist", ns))
		})
	}

	// 2. Verify excluded namespaces were NOT restored
	for _, ns := range w.namespacesToExclude {
		By(fmt.Sprintf("Checking excluded namespace %s does NOT exist", ns), func() {
			_, err := GetNamespace(w.Ctx, w.Client, ns)
			Expect(err).To(HaveOccurred(), fmt.Sprintf("Excluded namespace %s should NOT exist after restore", ns))
			Expect(apierrors.IsNotFound(err)).To(BeTrue(), "Error should be NotFound")
		})
	}
	return nil
}
