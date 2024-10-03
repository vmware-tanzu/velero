/*
Copyright The Velero Contributors.

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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/clock"
	testclocks "k8s.io/utils/clock/testing"
	ctrl "sigs.k8s.io/controller-runtime"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/vmware-tanzu/velero/internal/credentials"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/datapath"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

const name = "pvb-1"

func pvbBuilder() *builder.PodVolumeBackupBuilder {
	return builder.ForPodVolumeBackup(velerov1api.DefaultNamespace, name).
		PodNamespace(velerov1api.DefaultNamespace).
		PodName(name).
		Volume("pvb-1-volume").
		BackupStorageLocation("bsl-loc").
		ObjectMeta(
			func(obj metav1.Object) {
				obj.SetOwnerReferences([]metav1.OwnerReference{{Name: name}})
			},
		)
}

func podBuilder() *builder.PodBuilder {
	return builder.
		ForPod(velerov1api.DefaultNamespace, name).
		Volumes(&corev1.Volume{Name: "pvb-1-volume"})
}

func bslBuilder() *builder.BackupStorageLocationBuilder {
	return builder.
		ForBackupStorageLocation(velerov1api.DefaultNamespace, "bsl-loc")
}

func buildBackupRepo() *velerov1api.BackupRepository {
	return &velerov1api.BackupRepository{
		Spec: velerov1api.BackupRepositorySpec{ResticIdentifier: ""},
		TypeMeta: metav1.TypeMeta{
			APIVersion: velerov1api.SchemeGroupVersion.String(),
			Kind:       "BackupRepository",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: velerov1api.DefaultNamespace,
			Name:      fmt.Sprintf("%s-bsl-loc-restic-dn24h", velerov1api.DefaultNamespace),
			Labels: map[string]string{
				velerov1api.StorageLocationLabel: "bsl-loc",
				velerov1api.VolumeNamespaceLabel: velerov1api.DefaultNamespace,
				velerov1api.RepositoryTypeLabel:  "restic",
			},
		},
		Status: velerov1api.BackupRepositoryStatus{
			Phase: velerov1api.BackupRepositoryPhaseReady,
		},
	}
}

type fakeFSBR struct {
	pvb    *velerov1api.PodVolumeBackup
	client kbclient.Client
	clock  clock.WithTickerAndDelayedExecution
}

func (b *fakeFSBR) Init(ctx context.Context, param interface{}) error {
	return nil
}

func (b *fakeFSBR) StartBackup(source datapath.AccessPoint, uploaderConfigs map[string]string, param interface{}) error {
	pvb := b.pvb

	original := b.pvb.DeepCopy()
	pvb.Status.Phase = velerov1api.PodVolumeBackupPhaseCompleted
	pvb.Status.CompletionTimestamp = &metav1.Time{Time: b.clock.Now()}

	b.client.Patch(ctx, pvb, kbclient.MergeFrom(original))

	return nil
}

func (b *fakeFSBR) StartRestore(snapshotID string, target datapath.AccessPoint, uploaderConfigs map[string]string) error {
	return nil
}

func (b *fakeFSBR) Cancel() {
}

func (b *fakeFSBR) Close(ctx context.Context) {
}

var _ = Describe("PodVolumeBackup Reconciler", func() {
	type request struct {
		pvb               *velerov1api.PodVolumeBackup
		pod               *corev1.Pod
		bsl               *velerov1api.BackupStorageLocation
		backupRepo        *velerov1api.BackupRepository
		expectedProcessed bool
		expected          *velerov1api.PodVolumeBackup
		expectedRequeue   ctrl.Result
		expectedErrMsg    string
		dataMgr           *datapath.Manager
	}

	// `now` will be used to set the fake clock's time; capture
	// it here so it can be referenced in the test case defs.
	now, err := time.Parse(time.RFC1123, time.RFC1123)
	Expect(err).ToNot(HaveOccurred())
	now = now.Local()

	DescribeTable("a pod volume backup",
		func(test request) {
			ctx := context.Background()

			fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
			err = fakeClient.Create(ctx, test.pvb)
			Expect(err).ToNot(HaveOccurred())

			err = fakeClient.Create(ctx, test.pod)
			Expect(err).ToNot(HaveOccurred())

			err = fakeClient.Create(ctx, test.bsl)
			Expect(err).ToNot(HaveOccurred())

			err = fakeClient.Create(ctx, test.backupRepo)
			Expect(err).ToNot(HaveOccurred())

			fakeFS := velerotest.NewFakeFileSystem()
			pathGlob := fmt.Sprintf("/host_pods/%s/volumes/*/%s", "", "pvb-1-volume")
			_, err = fakeFS.Create(pathGlob)
			Expect(err).ToNot(HaveOccurred())

			credentialFileStore, err := credentials.NewNamespacedFileStore(
				fakeClient,
				velerov1api.DefaultNamespace,
				"/tmp/credentials",
				fakeFS,
			)

			Expect(err).ToNot(HaveOccurred())

			if test.dataMgr == nil {
				test.dataMgr = datapath.NewManager(1)
			}

			datapath.FSBRCreator = func(string, string, kbclient.Client, string, datapath.Callbacks, logrus.FieldLogger) datapath.AsyncBR {
				return &fakeFSBR{
					pvb:    test.pvb,
					client: fakeClient,
					clock:  testclocks.NewFakeClock(now),
				}
			}

			// Setup reconciler
			Expect(velerov1api.AddToScheme(scheme.Scheme)).To(Succeed())
			r := PodVolumeBackupReconciler{
				Client:           fakeClient,
				clock:            testclocks.NewFakeClock(now),
				metrics:          metrics.NewNodeMetrics(),
				credentialGetter: &credentials.CredentialGetter{FromFile: credentialFileStore},
				nodeName:         "test_node",
				fileSystem:       fakeFS,
				logger:           velerotest.NewLogger(),
				dataPathMgr:      test.dataMgr,
			}

			actualResult, err := r.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: velerov1api.DefaultNamespace,
					Name:      test.pvb.Name,
				},
			})
			Expect(actualResult).To(BeEquivalentTo(test.expectedRequeue))
			if test.expectedErrMsg == "" {
				Expect(err).ToNot(HaveOccurred())
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
				Expect(err).ToNot(HaveOccurred())
				Eventually(pvb.Status.Phase).Should(Equal(test.expected.Status.Phase))
			}

			// Processed PVBs will have completion timestamps.
			if test.expectedProcessed == true {
				Expect(pvb.Status.CompletionTimestamp).ToNot(BeNil())
			}

			// Unprocessed PVBs will not have completion timestamps.
			if test.expectedProcessed == false {
				Expect(pvb.Status.CompletionTimestamp).To(BeNil())
			}
		},
		Entry("empty phase pvb on same node should be processed", request{
			pvb:               pvbBuilder().Phase("").Node("test_node").Result(),
			pod:               podBuilder().Result(),
			bsl:               bslBuilder().Result(),
			backupRepo:        buildBackupRepo(),
			expectedProcessed: true,
			expected: builder.ForPodVolumeBackup(velerov1api.DefaultNamespace, "pvb-1").
				Phase(velerov1api.PodVolumeBackupPhaseCompleted).
				Result(),
			expectedRequeue: ctrl.Result{},
		}),
		Entry("new phase pvb on same node should be processed", request{
			pvb: pvbBuilder().
				Phase(velerov1api.PodVolumeBackupPhaseNew).
				Node("test_node").
				Result(),
			pod:               podBuilder().Result(),
			bsl:               bslBuilder().Result(),
			backupRepo:        buildBackupRepo(),
			expectedProcessed: true,
			expected: builder.ForPodVolumeBackup(velerov1api.DefaultNamespace, "pvb-1").
				Phase(velerov1api.PodVolumeBackupPhaseCompleted).
				Result(),
			expectedRequeue: ctrl.Result{},
		}),
		Entry("in progress phase pvb on same node should not be processed", request{
			pvb: pvbBuilder().
				Phase(velerov1api.PodVolumeBackupPhaseInProgress).
				Node("test_node").
				Result(),
			pod:               podBuilder().Result(),
			bsl:               bslBuilder().Result(),
			backupRepo:        buildBackupRepo(),
			expectedProcessed: false,
			expected: builder.ForPodVolumeBackup(velerov1api.DefaultNamespace, "pvb-1").
				Phase(velerov1api.PodVolumeBackupPhaseInProgress).
				Result(),
			expectedRequeue: ctrl.Result{},
		}),
		Entry("completed phase pvb on same node should not be processed", request{
			pvb: pvbBuilder().
				Phase(velerov1api.PodVolumeBackupPhaseCompleted).
				Node("test_node").
				Result(),
			pod:               podBuilder().Result(),
			bsl:               bslBuilder().Result(),
			backupRepo:        buildBackupRepo(),
			expectedProcessed: false,
			expected: builder.ForPodVolumeBackup(velerov1api.DefaultNamespace, "pvb-1").
				Phase(velerov1api.PodVolumeBackupPhaseCompleted).
				Result(),
			expectedRequeue: ctrl.Result{},
		}),
		Entry("failed phase pvb on same node should not be processed", request{
			pvb: pvbBuilder().
				Phase(velerov1api.PodVolumeBackupPhaseFailed).
				Node("test_node").
				Result(),
			pod:               podBuilder().Result(),
			bsl:               bslBuilder().Result(),
			backupRepo:        buildBackupRepo(),
			expectedProcessed: false,
			expected: builder.ForPodVolumeBackup(velerov1api.DefaultNamespace, "pvb-1").
				Phase(velerov1api.PodVolumeBackupPhaseFailed).
				Result(),
			expectedRequeue: ctrl.Result{},
		}),
		Entry("empty phase pvb on different node should not be processed", request{
			pvb: pvbBuilder().
				Phase(velerov1api.PodVolumeBackupPhaseFailed).
				Node("test_node_2").
				Result(),
			pod:               podBuilder().Result(),
			bsl:               bslBuilder().Result(),
			backupRepo:        buildBackupRepo(),
			expectedProcessed: false,
			expected: builder.ForPodVolumeBackup(velerov1api.DefaultNamespace, "pvb-1").
				Phase(velerov1api.PodVolumeBackupPhaseFailed).
				Result(),
			expectedRequeue: ctrl.Result{},
		}),
		Entry("new phase pvb on different node should not be processed", request{
			pvb: pvbBuilder().
				Phase(velerov1api.PodVolumeBackupPhaseNew).
				Node("test_node_2").
				Result(),
			pod:               podBuilder().Result(),
			bsl:               bslBuilder().Result(),
			backupRepo:        buildBackupRepo(),
			expectedProcessed: false,
			expected: builder.ForPodVolumeBackup(velerov1api.DefaultNamespace, "pvb-1").
				Phase(velerov1api.PodVolumeBackupPhaseNew).
				Result(),
			expectedRequeue: ctrl.Result{},
		}),
		Entry("in progress phase pvb on different node should not be processed", request{
			pvb: pvbBuilder().
				Phase(velerov1api.PodVolumeBackupPhaseInProgress).
				Node("test_node_2").
				Result(),
			pod:               podBuilder().Result(),
			bsl:               bslBuilder().Result(),
			backupRepo:        buildBackupRepo(),
			expectedProcessed: false,
			expected: builder.ForPodVolumeBackup(velerov1api.DefaultNamespace, "pvb-1").
				Phase(velerov1api.PodVolumeBackupPhaseInProgress).
				Result(),
			expectedRequeue: ctrl.Result{},
		}),
		Entry("completed phase pvb on different node should not be processed", request{
			pvb: pvbBuilder().
				Phase(velerov1api.PodVolumeBackupPhaseCompleted).
				Node("test_node_2").
				Result(),
			pod:               podBuilder().Result(),
			bsl:               bslBuilder().Result(),
			backupRepo:        buildBackupRepo(),
			expectedProcessed: false,
			expected: builder.ForPodVolumeBackup(velerov1api.DefaultNamespace, "pvb-1").
				Phase(velerov1api.PodVolumeBackupPhaseCompleted).
				Result(),
			expectedRequeue: ctrl.Result{},
		}),
		Entry("failed phase pvb on different node should not be processed", request{
			pvb: pvbBuilder().
				Phase(velerov1api.PodVolumeBackupPhaseFailed).
				Node("test_node_2").
				Result(),
			pod:               podBuilder().Result(),
			bsl:               bslBuilder().Result(),
			backupRepo:        buildBackupRepo(),
			expectedProcessed: false,
			expected: builder.ForPodVolumeBackup(velerov1api.DefaultNamespace, "pvb-1").
				Phase(velerov1api.PodVolumeBackupPhaseFailed).
				Result(),
			expectedRequeue: ctrl.Result{},
		}),
		Entry("pvb should be requeued when exceeding max concurrent number", request{
			pvb:               pvbBuilder().Phase("").Node("test_node").Result(),
			pod:               podBuilder().Result(),
			bsl:               bslBuilder().Result(),
			backupRepo:        buildBackupRepo(),
			dataMgr:           datapath.NewManager(0),
			expectedProcessed: false,
			expected: builder.ForPodVolumeBackup(velerov1api.DefaultNamespace, "pvb-1").
				Phase("").
				Result(),
			expectedRequeue: ctrl.Result{Requeue: true, RequeueAfter: time.Second * 5},
		}),
	)
})
