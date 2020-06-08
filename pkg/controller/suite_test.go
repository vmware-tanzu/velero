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

package controller

import (
	"testing"

	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"

	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// +kubebuilder:scaffold:imports
	// +kubebuilder:scaffold:imports
)

func newFakeClient(t *testing.T, initObjs ...runtime.Object) client.Client {
	err := velerov1api.AddToScheme(scheme.Scheme)
	require.NoError(t, err)
	return fake.NewFakeClientWithScheme(scheme.Scheme, initObjs...)
}

/*


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

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.
//
//var cfg *rest.Config
//var k8sClient client.Client
//var testEnv *envtest.Environment
//
//func TestAPIs(t *testing.T) {
//	RegisterFailHandler(Fail)
//
//	RunSpecsWithDefaultAndCustomReporters(t,
//		"Controller Suite",
//		[]Reporter{printer.NewlineReporter{}})
//}
//
//var _ = BeforeSuite(func(done Done) {
//	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))
//
//	By("bootstrapping test environment")
//	testEnv = &envtest.Environment{
//		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
//	}
//
//	var err error
//	cfg, err = testEnv.Start()
//	Expect(err).ToNot(HaveOccurred())
//	Expect(cfg).ToNot(BeNil())
//
//	err = velerov1api.AddToScheme(scheme.Scheme)
//	Expect(err).NotTo(HaveOccurred())
//
//	// +kubebuilder:scaffold:scheme
//
//	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
//	Expect(err).ToNot(HaveOccurred())
//	Expect(k8sClient).ToNot(BeNil())
//
//	close(done)
//}, 60)
//
//var _ = AfterSuite(func() {
//	By("tearing down the test environment")
//	err := testEnv.Stop()
//	Expect(err).ToNot(HaveOccurred())
//})
