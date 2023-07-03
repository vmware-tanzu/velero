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
	"testing"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgofake "k8s.io/client-go/kubernetes/fake"
	ctrl "sigs.k8s.io/controller-runtime"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/internal/credentials"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov2alpha1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v2alpha1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/datapath"
	"github.com/vmware-tanzu/velero/pkg/exposer"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	datapathmockes "github.com/vmware-tanzu/velero/pkg/datapath/mocks"
	exposermockes "github.com/vmware-tanzu/velero/pkg/exposer/mocks"
)

const dataDownloadName string = "datadownload-1"

func dataDownloadBuilder() *builder.DataDownloadBuilder {
	return builder.ForDataDownload(velerov1api.DefaultNamespace, dataDownloadName).
		BackupStorageLocation("bsl-loc").
		DataMover("velero").
		SnapshotID("test-snapshot-id").TargetVolume(velerov2alpha1api.TargetVolumeSpec{
		PV:        "test-pv",
		PVC:       "test-pvc",
		Namespace: "test-ns",
	})
}

func initDataDownloadReconciler(objects []runtime.Object, needError ...bool) (*DataDownloadReconciler, error) {
	scheme := runtime.NewScheme()
	err := velerov1api.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}
	err = velerov2alpha1api.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}
	err = corev1.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}

	fakeClient := &FakeClient{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
	}

	if len(needError) == 4 {
		fakeClient.getError = needError[0]
		fakeClient.createError = needError[1]
		fakeClient.updateError = needError[2]
		fakeClient.patchError = needError[3]
	}

	fakeKubeClient := clientgofake.NewSimpleClientset(objects...)
	fakeFS := velerotest.NewFakeFileSystem()
	pathGlob := fmt.Sprintf("/host_pods/%s/volumes/*/%s", "", dataDownloadName)
	_, err = fakeFS.Create(pathGlob)
	if err != nil {
		return nil, err
	}

	credentialFileStore, err := credentials.NewNamespacedFileStore(
		fakeClient,
		velerov1api.DefaultNamespace,
		"/tmp/credentials",
		fakeFS,
	)
	if err != nil {
		return nil, err
	}
	return NewDataDownloadReconciler(fakeClient, fakeKubeClient, nil, &credentials.CredentialGetter{FromFile: credentialFileStore}, "test_node", velerotest.NewLogger()), nil
}

func TestDataDownloadReconcile(t *testing.T) {
	tests := []struct {
		name              string
		dd                *velerov2alpha1api.DataDownload
		targetPVC         *corev1.PersistentVolumeClaim
		dataMgr           *datapath.Manager
		needErrs          []bool
		isExposeErr       bool
		isGetExposeErr    bool
		expectedStatusMsg string
	}{
		{
			name:      "Restore is exposed",
			dd:        dataDownloadBuilder().Result(),
			targetPVC: builder.ForPersistentVolumeClaim("test-ns", "test-pvc").Result(),
		},
		{
			name:      "Get empty restore exposer",
			dd:        dataDownloadBuilder().Phase(velerov2alpha1api.DataDownloadPhasePrepared).Result(),
			targetPVC: builder.ForPersistentVolumeClaim("test-ns", "test-pvc").Result(),
		},
		{
			name:              "Failed to get restore exposer",
			dd:                dataDownloadBuilder().Phase(velerov2alpha1api.DataDownloadPhasePrepared).Result(),
			targetPVC:         builder.ForPersistentVolumeClaim("test-ns", "test-pvc").Result(),
			expectedStatusMsg: "Error to get restore exposer",
			isGetExposeErr:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, err := initDataDownloadReconciler([]runtime.Object{test.targetPVC}, test.needErrs...)
			require.NoError(t, err)
			defer func() {
				r.client.Delete(ctx, test.dd, &kbclient.DeleteOptions{})
				if test.targetPVC != nil {
					r.client.Delete(ctx, test.targetPVC, &kbclient.DeleteOptions{})
				}
			}()

			ctx := context.Background()
			if test.dd.Namespace == velerov1api.DefaultNamespace {
				err = r.client.Create(ctx, test.dd)
				require.NoError(t, err)
			}

			if test.dataMgr != nil {
				r.dataPathMgr = test.dataMgr
			} else {
				r.dataPathMgr = datapath.NewManager(1)
			}

			datapath.FSBRCreator = func(string, string, kbclient.Client, string, datapath.Callbacks, logrus.FieldLogger) datapath.AsyncBR {
				return datapathmockes.NewAsyncBR(t)
			}

			if test.isExposeErr || test.isGetExposeErr {
				r.restoreExposer = func() exposer.GenericRestoreExposer {
					ep := exposermockes.NewGenericRestoreExposer(t)
					if test.isExposeErr {
						ep.On("Expose", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("Error to expose restore exposer"))
					}

					if test.isGetExposeErr {
						ep.On("GetExposed", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("Error to get restore exposer"))
					}

					ep.On("CleanUp", mock.Anything, mock.Anything).Return()
					return ep
				}()
			}

			if test.dd.Status.Phase == velerov2alpha1api.DataDownloadPhaseInProgress {
				if fsBR := r.dataPathMgr.GetAsyncBR(test.dd.Name); fsBR == nil {
					_, err := r.dataPathMgr.CreateFileSystemBR(test.dd.Name, pVBRRequestor, ctx, r.client, velerov1api.DefaultNamespace, datapath.Callbacks{OnCancelled: r.OnDataDownloadCancelled}, velerotest.NewLogger())
					require.NoError(t, err)
				}
			}
			actualResult, err := r.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: velerov1api.DefaultNamespace,
					Name:      test.dd.Name,
				},
			})

			if test.isGetExposeErr {
				assert.Contains(t, err.Error(), test.expectedStatusMsg)
			} else {
				require.Nil(t, err)
			}

			require.NotNil(t, actualResult)

			dd := velerov2alpha1api.DataDownload{}
			err = r.client.Get(ctx, kbclient.ObjectKey{
				Name:      test.dd.Name,
				Namespace: test.dd.Namespace,
			}, &dd)

			if test.isGetExposeErr {
				assert.Contains(t, dd.Status.Message, test.expectedStatusMsg)
			}
			require.Nil(t, err)
			t.Logf("%s: \n %v \n", test.name, dd)
		})
	}
}
