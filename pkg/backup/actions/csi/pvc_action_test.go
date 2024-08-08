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

package csi

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v7/apis/volumesnapshot/v1"
	v1 "github.com/kubernetes-csi/external-snapshotter/client/v7/apis/volumesnapshot/v1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/pkg/apis/velero/shared"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov2alpha1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v2alpha1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	factorymocks "github.com/vmware-tanzu/velero/pkg/client/mocks"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
)

func TestExecute(t *testing.T) {
	boolTrue := true
	tests := []struct {
		name               string
		backup             *velerov1api.Backup
		pvc                *corev1.PersistentVolumeClaim
		pv                 *corev1.PersistentVolume
		sc                 *storagev1.StorageClass
		vsClass            *snapshotv1api.VolumeSnapshotClass
		operationID        string
		expectedErr        error
		expectedBackup     *velerov1api.Backup
		expectedDataUpload *velerov2alpha1.DataUpload
		expectedPVC        *corev1.PersistentVolumeClaim
		resourcePolicy     *corev1.ConfigMap
	}{
		{
			name:        "Skip PVC BIA when backup is in finalizing phase",
			backup:      builder.ForBackup("velero", "test").Phase(velerov1api.BackupPhaseFinalizing).Result(),
			expectedErr: nil,
		},
		{
			name:        "Test SnapshotMoveData",
			backup:      builder.ForBackup("velero", "test").SnapshotMoveData(true).CSISnapshotTimeout(1 * time.Minute).Result(),
			pvc:         builder.ForPersistentVolumeClaim("velero", "testPVC").VolumeName("testPV").StorageClass("testSC").Phase(corev1.ClaimBound).Result(),
			pv:          builder.ForPersistentVolume("testPV").CSI("hostpath", "testVolume").Result(),
			sc:          builder.ForStorageClass("testSC").Provisioner("hostpath").Result(),
			vsClass:     builder.ForVolumeSnapshotClass("testVSClass").Driver("hostpath").ObjectMeta(builder.WithLabels(velerov1api.VolumeSnapshotClassSelectorLabel, "")).Result(),
			operationID: ".",
			expectedErr: nil,
			expectedDataUpload: &velerov2alpha1.DataUpload{
				TypeMeta: metav1.TypeMeta{
					Kind:       "DataUpload",
					APIVersion: velerov2alpha1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-",
					Namespace:    "velero",
					Labels: map[string]string{
						velerov1api.BackupNameLabel:       "test",
						velerov1api.BackupUIDLabel:        "",
						velerov1api.PVCUIDLabel:           "",
						velerov1api.AsyncOperationIDLabel: "du-.",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "velero.io/v1",
							Kind:       "Backup",
							Name:       "test",
							UID:        "",
							Controller: &boolTrue,
						},
					},
				},
				Spec: velerov2alpha1.DataUploadSpec{
					SnapshotType: velerov2alpha1.SnapshotTypeCSI,
					CSISnapshot: &velerov2alpha1.CSISnapshotSpec{
						VolumeSnapshot: "",
						StorageClass:   "testSC",
						SnapshotClass:  "testVSClass",
					},
					SourcePVC:        "testPVC",
					SourceNamespace:  "velero",
					OperationTimeout: metav1.Duration{Duration: 1 * time.Minute},
				},
			},
		},
		{
			name:        "Verify PVC is modified as expected",
			backup:      builder.ForBackup("velero", "test").SnapshotMoveData(true).CSISnapshotTimeout(1 * time.Minute).Result(),
			pvc:         builder.ForPersistentVolumeClaim("velero", "testPVC").VolumeName("testPV").StorageClass("testSC").Phase(corev1.ClaimBound).Result(),
			pv:          builder.ForPersistentVolume("testPV").CSI("hostpath", "testVolume").Result(),
			sc:          builder.ForStorageClass("testSC").Provisioner("hostpath").Result(),
			vsClass:     builder.ForVolumeSnapshotClass("tescVSClass").Driver("hostpath").ObjectMeta(builder.WithLabels(velerov1api.VolumeSnapshotClassSelectorLabel, "")).Result(),
			operationID: ".",
			expectedErr: nil,
			expectedPVC: builder.ForPersistentVolumeClaim("velero", "testPVC").
				ObjectMeta(builder.WithAnnotations(velerov1api.MustIncludeAdditionalItemAnnotation, "true", velerov1api.DataUploadNameAnnotation, "velero/", velerov1api.VolumeSnapshotLabel, ""),
					builder.WithLabels(velerov1api.BackupNameLabel, "test", velerov1api.VolumeSnapshotLabel, "")).
				VolumeName("testPV").StorageClass("testSC").Phase(corev1.ClaimBound).Result(),
		},
		{
			name:           "Test ResourcePolicy",
			backup:         builder.ForBackup("velero", "test").ResourcePolicies("resourcePolicy").SnapshotVolumes(false).Result(),
			resourcePolicy: builder.ForConfigMap("velero", "resourcePolicy").Data("policy", "{\"version\":\"v1\", \"volumePolicies\":[{\"conditions\":{\"csi\": {}},\"action\":{\"type\":\"snapshot\"}}]}").Result(),
			pvc:            builder.ForPersistentVolumeClaim("velero", "testPVC").VolumeName("testPV").StorageClass("testSC").Phase(corev1.ClaimBound).Result(),
			pv:             builder.ForPersistentVolume("testPV").CSI("hostpath", "testVolume").Result(),
			sc:             builder.ForStorageClass("testSC").Provisioner("hostpath").Result(),
			vsClass:        builder.ForVolumeSnapshotClass("tescVSClass").Driver("hostpath").ObjectMeta(builder.WithLabels(velerov1api.VolumeSnapshotClassSelectorLabel, "")).Result(),
			expectedErr:    nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(*testing.T) {
			crClient := velerotest.NewFakeControllerRuntimeClient(t)
			logger := logrus.New()
			logger.Level = logrus.DebugLevel

			if tc.pvc != nil {
				require.NoError(t, crClient.Create(context.Background(), tc.pvc))
			}
			if tc.pv != nil {
				require.NoError(t, crClient.Create(context.Background(), tc.pv))
			}
			if tc.sc != nil {
				require.NoError(t, crClient.Create(context.Background(), tc.sc))
			}
			if tc.vsClass != nil {
				require.NoError(t, crClient.Create(context.Background(), tc.vsClass))
			}
			if tc.resourcePolicy != nil {
				require.NoError(t, crClient.Create(context.Background(), tc.resourcePolicy))
			}

			pvcBIA := pvcBackupItemAction{
				log:      logger,
				crClient: crClient,
			}

			pvcMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&tc.pvc)
			require.NoError(t, err)

			if boolptr.IsSetToTrue(tc.backup.Spec.SnapshotMoveData) == true {
				go func() {
					var vsList v1.VolumeSnapshotList
					err := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 10*time.Second, true, func(ctx context.Context) (bool, error) {
						err = pvcBIA.crClient.List(ctx, &vsList, &crclient.ListOptions{Namespace: tc.pvc.Namespace})
						//nolint:testifylint // false-positive
						assert.NoError(t, err)
						if err != nil || len(vsList.Items) == 0 {
							//lint:ignore nilerr reason
							return false, nil // ignore
						}
						return true, nil
					})

					assert.NoError(t, err)
					vscName := "testVSC"
					readyToUse := true
					vsList.Items[0].Status = &v1.VolumeSnapshotStatus{
						BoundVolumeSnapshotContentName: &vscName,
						ReadyToUse:                     &readyToUse,
					}
					err = pvcBIA.crClient.Update(context.Background(), &vsList.Items[0])
					assert.NoError(t, err)

					handleName := "testHandle"
					vsc := builder.ForVolumeSnapshotContent("testVSC").Status(&snapshotv1api.VolumeSnapshotContentStatus{SnapshotHandle: &handleName}).Result()
					err = pvcBIA.crClient.Create(context.Background(), vsc)
					assert.NoError(t, err)
				}()
			}

			resultUnstructed, _, _, _, err := pvcBIA.Execute(&unstructured.Unstructured{Object: pvcMap}, tc.backup)
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}

			if tc.expectedDataUpload != nil {
				dataUploadList := new(velerov2alpha1.DataUploadList)
				err := crClient.List(context.Background(), dataUploadList, &crclient.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{velerov1api.BackupNameLabel: tc.backup.Name})})
				require.NoError(t, err)
				require.Len(t, dataUploadList.Items, 1)
				require.True(t, cmp.Equal(tc.expectedDataUpload, &dataUploadList.Items[0], cmpopts.IgnoreFields(velerov2alpha1.DataUpload{}, "ResourceVersion", "Name", "Spec.CSISnapshot.VolumeSnapshot")))
			}

			if tc.expectedPVC != nil {
				resultPVC := new(corev1.PersistentVolumeClaim)
				runtime.DefaultUnstructuredConverter.FromUnstructured(resultUnstructed.UnstructuredContent(), resultPVC)

				require.True(t, cmp.Equal(tc.expectedPVC, resultPVC, cmpopts.IgnoreFields(corev1.PersistentVolumeClaim{}, "ResourceVersion", "Annotations", "Labels")))
			}
		})
	}
}

func TestProgress(t *testing.T) {
	currentTime := time.Now()
	tests := []struct {
		name             string
		backup           *velerov1api.Backup
		dataUpload       *velerov2alpha1.DataUpload
		operationID      string
		expectedErr      string
		expectedProgress velero.OperationProgress
	}{
		{
			name:        "DataUpload cannot be found",
			backup:      builder.ForBackup("velero", "test").Result(),
			operationID: "testing",
			expectedErr: "not found DataUpload for operationID testing",
		},
		{
			name:   "DataUpload is found",
			backup: builder.ForBackup("velero", "test").Result(),
			dataUpload: &velerov2alpha1.DataUpload{
				TypeMeta: metav1.TypeMeta{
					Kind:       "DataUpload",
					APIVersion: "v2alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "velero",
					Name:      "testing",
					Labels: map[string]string{
						velerov1api.AsyncOperationIDLabel: "testing",
					},
				},
				Status: velerov2alpha1.DataUploadStatus{
					Phase: velerov2alpha1.DataUploadPhaseFailed,
					Progress: shared.DataMoveOperationProgress{
						BytesDone:  1000,
						TotalBytes: 1000,
					},
					StartTimestamp:      &metav1.Time{Time: currentTime},
					CompletionTimestamp: &metav1.Time{Time: currentTime},
					Message:             "Testing error",
				},
			},
			operationID: "testing",
			expectedProgress: velero.OperationProgress{
				Completed:      true,
				Err:            "Testing error",
				NCompleted:     1000,
				NTotal:         1000,
				OperationUnits: "Bytes",
				Description:    "Failed",
				Started:        currentTime,
				Updated:        currentTime,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(*testing.T) {
			crClient := velerotest.NewFakeControllerRuntimeClient(t)
			logger := logrus.New()

			pvcBIA := pvcBackupItemAction{
				log:      logger,
				crClient: crClient,
			}

			if tc.dataUpload != nil {
				err := crClient.Create(context.Background(), tc.dataUpload)
				require.NoError(t, err)
			}

			progress, err := pvcBIA.Progress(tc.operationID, tc.backup)
			if tc.expectedErr != "" {
				require.Equal(t, tc.expectedErr, err.Error())
			}
			require.True(t, cmp.Equal(tc.expectedProgress, progress, cmpopts.IgnoreFields(velero.OperationProgress{}, "Started", "Updated")))
		})
	}
}

func TestCancel(t *testing.T) {
	tests := []struct {
		name               string
		backup             *velerov1api.Backup
		dataUpload         velerov2alpha1.DataUpload
		operationID        string
		expectedErr        error
		expectedDataUpload velerov2alpha1.DataUpload
	}{
		{
			name:   "Cancel DataUpload",
			backup: builder.ForBackup("velero", "test").Result(),
			dataUpload: velerov2alpha1.DataUpload{
				TypeMeta: metav1.TypeMeta{
					Kind:       "DataUpload",
					APIVersion: velerov2alpha1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "velero",
					Name:      "testing",
					Labels: map[string]string{
						velerov1api.AsyncOperationIDLabel: "testing",
					},
				},
			},
			operationID: "testing",
			expectedErr: nil,
			expectedDataUpload: velerov2alpha1.DataUpload{
				TypeMeta: metav1.TypeMeta{
					Kind:       "DataUpload",
					APIVersion: velerov2alpha1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "velero",
					Name:      "testing",
					Labels: map[string]string{
						velerov1api.AsyncOperationIDLabel: "testing",
					},
				},
				Spec: velerov2alpha1.DataUploadSpec{
					Cancel: true,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(*testing.T) {
			crClient := velerotest.NewFakeControllerRuntimeClient(t)
			logger := logrus.New()

			pvcBIA := pvcBackupItemAction{
				log:      logger,
				crClient: crClient,
			}

			err := crClient.Create(context.Background(), &tc.dataUpload)
			require.NoError(t, err)

			err = pvcBIA.Cancel(tc.operationID, tc.backup)
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			}

			du := new(velerov2alpha1.DataUpload)
			err = crClient.Get(context.Background(), crclient.ObjectKey{Namespace: tc.dataUpload.Namespace, Name: tc.dataUpload.Name}, du)
			require.NoError(t, err)

			require.True(t, cmp.Equal(tc.expectedDataUpload, *du, cmpopts.IgnoreFields(velerov2alpha1.DataUpload{}, "ResourceVersion")))
		})
	}
}

func TestPVCAppliesTo(t *testing.T) {
	p := pvcBackupItemAction{
		log: logrus.StandardLogger(),
	}
	selector, err := p.AppliesTo()

	require.NoError(t, err)

	require.Equal(
		t,
		velero.ResourceSelector{
			IncludedResources: []string{"persistentvolumeclaims"},
		},
		selector,
	)
}

func TestNewPVCBackupItemAction(t *testing.T) {
	logger := logrus.StandardLogger()
	crClient := velerotest.NewFakeControllerRuntimeClient(t)

	f := &factorymocks.Factory{}
	f.On("KubebuilderClient").Return(nil, fmt.Errorf(""))
	plugin := NewPvcBackupItemAction(f)
	_, err := plugin(logger)
	require.Error(t, err)

	f1 := &factorymocks.Factory{}
	f1.On("KubebuilderClient").Return(crClient, nil)
	plugin1 := NewPvcBackupItemAction(f1)
	_, err1 := plugin1(logger)
	require.NoError(t, err1)
}
