/*
Copyright The Velero contributors.

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
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

const (
	name = "pvb-1"
)

func pvbBuilder() *builder.PodVolumeBackupBuilder {
	return builder.ForPodVolumeBackup(velerov1api.DefaultNamespace, name)
}

func podBuilder() *builder.PodBuilder {
	return builder.ForPod(velerov1api.DefaultNamespace, name)
}

var _ = Describe("Pod Volume Backup Reconciler", func() {
	type request struct {
		pvb *velerov1api.PodVolumeBackup
		pod *corev1.Pod

		expected        *velerov1api.PodVolumeBackup
		expectedRequeue ctrl.Result
		expectedErrMsg  string
	}

	// `now` will be used to set the fake clock's time; capture
	// it here so it can be referenced in the test case defs.
	now, err := time.Parse(time.RFC1123, time.RFC1123)
	Expect(err).To(BeNil())
	now = now.Local()

	DescribeTable("a pod volume backup",
		func(test request) {
			// Setup reconciler
			Expect(velerov1api.AddToScheme(scheme.Scheme)).To(Succeed())
			r := PodVolumeBackupReconciler{
				Client:     fake.NewFakeClientWithScheme(scheme.Scheme, test.pvb),
				Ctx:        context.Background(),
				Clock:      clock.NewFakeClock(now),
				Metrics:    metrics.NewResticServerMetrics(),
				NodeName:   "foo",
				FileSystem: velerotest.NewFakeFileSystem(),
				Log:        velerotest.NewLogger(),
			}

			err := r.Client.Create(r.Ctx, test.pod)
			Expect(err).To(BeNil())

			pathGlob := fmt.Sprintf("/host_pods/%s/volumes/*/%s", "", "pvb-1-volume")
			r.FileSystem.Create(pathGlob)

			actualResult, err := r.Reconcile(r.Ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: velerov1api.DefaultNamespace,
					Name:      test.pvb.Name,
				},
			})

			Expect(actualResult).To(BeEquivalentTo(test.expectedRequeue))
			if test.expectedErrMsg == "" {
				Expect(err).To(BeNil())
			} else {
				Expect(err.Error()).To(BeEquivalentTo(test.expectedErrMsg))
			}

			pvb := velerov1api.PodVolumeBackup{}
			err = r.Client.Get(ctx, kbclient.ObjectKey{
				Name:      test.pvb.Name,
				Namespace: test.pvb.Namespace,
			}, &pvb)

			// Assertions
			if test.expected == nil {
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			} else {
				Expect(err).To(BeNil())
				Eventually(pvb.Status.Phase).Should(Equal(test.expected.Status.Phase))
			}
		},
		Entry("with with phase=empty will be processed and phase successfully patched", request{
			pvb: pvbBuilder().
				Node("foo").
				PodName(name).
				PodNamespace(velerov1api.DefaultNamespace).
				Volume("pvb-1-volume").
				Result(),
			pod: podBuilder().
				Volumes(&corev1.Volume{Name: "pvb-1-volume"}).
				Result(),
			expected: pvbBuilder().
				Phase(velerov1api.PodVolumeBackupPhaseNew).
				Result(),
			expectedRequeue: ctrl.Result{},
		}),
	)
})
