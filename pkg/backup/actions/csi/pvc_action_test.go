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
	"github.com/stretchr/testify/require"
	corev1api "k8s.io/api/core/v1"
	storagev1api "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/vmware-tanzu/velero/pkg/apis/velero/shared"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov2alpha1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v2alpha1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	factorymocks "github.com/vmware-tanzu/velero/pkg/client/mocks"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestExecute(t *testing.T) {
	boolTrue := true
	tests := []struct {
		name               string
		backup             *velerov1api.Backup
		pvc                *corev1api.PersistentVolumeClaim
		pv                 *corev1api.PersistentVolume
		sc                 *storagev1api.StorageClass
		vsClass            *snapshotv1api.VolumeSnapshotClass
		operationID        string
		expectedErr        error
		expectedBackup     *velerov1api.Backup
		expectedDataUpload *velerov2alpha1.DataUpload
		expectedPVC        *corev1api.PersistentVolumeClaim
		resourcePolicy     *corev1api.ConfigMap
		additionalItems    []velero.ResourceIdentifier
	}{
		{
			name:        "Skip PVC BIA when backup is in finalizing phase",
			backup:      builder.ForBackup("velero", "test").Phase(velerov1api.BackupPhaseFinalizing).Result(),
			expectedErr: nil,
		},
		{
			name:        "Test SnapshotMoveData",
			backup:      builder.ForBackup("velero", "test").SnapshotMoveData(true).CSISnapshotTimeout(1 * time.Minute).Result(),
			pvc:         builder.ForPersistentVolumeClaim("velero", "testPVC").VolumeName("testPV").StorageClass("testSC").Phase(corev1api.ClaimBound).Result(),
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
			pvc:         builder.ForPersistentVolumeClaim("velero", "testPVC").VolumeName("testPV").StorageClass("testSC").Phase(corev1api.ClaimBound).Result(),
			pv:          builder.ForPersistentVolume("testPV").CSI("hostpath", "testVolume").Result(),
			sc:          builder.ForStorageClass("testSC").Provisioner("hostpath").Result(),
			vsClass:     builder.ForVolumeSnapshotClass("tescVSClass").Driver("hostpath").ObjectMeta(builder.WithLabels(velerov1api.VolumeSnapshotClassSelectorLabel, "")).Result(),
			operationID: ".",
			expectedErr: nil,
			expectedPVC: builder.ForPersistentVolumeClaim("velero", "testPVC").
				ObjectMeta(builder.WithAnnotations(velerov1api.MustIncludeAdditionalItemAnnotation, "true", velerov1api.DataUploadNameAnnotation, "velero/"),
					builder.WithLabels(velerov1api.BackupNameLabel, "test")).
				VolumeName("testPV").StorageClass("testSC").Phase(corev1api.ClaimBound).Result(),
		},
		{
			name:           "Test ResourcePolicy",
			backup:         builder.ForBackup("velero", "test").ResourcePolicies("resourcePolicy").SnapshotVolumes(false).CSISnapshotTimeout(time.Duration(3600) * time.Second).Result(),
			resourcePolicy: builder.ForConfigMap("velero", "resourcePolicy").Data("policy", "{\"version\":\"v1\", \"volumePolicies\":[{\"conditions\":{\"csi\": {}},\"action\":{\"type\":\"snapshot\"}}]}").Result(),
			pvc:            builder.ForPersistentVolumeClaim("velero", "testPVC").VolumeName("testPV").StorageClass("testSC").Phase(corev1api.ClaimBound).Result(),
			pv:             builder.ForPersistentVolume("testPV").CSI("hostpath", "testVolume").Result(),
			sc:             builder.ForStorageClass("testSC").Provisioner("hostpath").Result(),
			vsClass:        builder.ForVolumeSnapshotClass("tescVSClass").Driver("hostpath").ObjectMeta(builder.WithLabels(velerov1api.VolumeSnapshotClassSelectorLabel, "")).Result(),
			expectedErr:    nil,
		},
		{
			name:        "Test pure CSI snapshot path (no data mover)",
			backup:      builder.ForBackup("velero", "test").SnapshotMoveData(false).CSISnapshotTimeout(1 * time.Minute).Result(),
			pvc:         builder.ForPersistentVolumeClaim("velero", "testPVC").VolumeName("testPV").StorageClass("testSC").Phase(corev1api.ClaimBound).Result(),
			pv:          builder.ForPersistentVolume("testPV").CSI("hostpath", "testVolume").Result(),
			sc:          builder.ForStorageClass("testSC").Provisioner("hostpath").Result(),
			vsClass:     builder.ForVolumeSnapshotClass("testVSClass").Driver("hostpath").ObjectMeta(builder.WithLabels(velerov1api.VolumeSnapshotClassSelectorLabel, "")).Result(),
			expectedErr: nil,
			additionalItems: []velero.ResourceIdentifier{
				{
					GroupResource: kuberesource.VolumeSnapshots,
					Namespace:     "velero",
					Name:          "velero-testPVC-", // name is generated
				},
			},
			expectedPVC: builder.ForPersistentVolumeClaim("velero", "testPVC").
				ObjectMeta(builder.WithAnnotations(velerov1api.MustIncludeAdditionalItemAnnotation, "true", velerov1api.VolumeSnapshotLabel, "velero-testPVC-"),
					builder.WithLabels(velerov1api.BackupNameLabel, "test", velerov1api.VolumeSnapshotLabel, "velero-testPVC-")).
				VolumeName("testPV").StorageClass("testSC").Phase(corev1api.ClaimBound).Result(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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
			var pvcMap map[string]any
			var err error
			if tc.pvc != nil {
				pvcMap, err = runtime.DefaultUnstructuredConverter.ToUnstructured(tc.pvc)
				require.NoError(t, err)
			} else {
				pvcMap = make(map[string]any)
			}

			if tc.pvc != nil {
				go func() {
					var vsList v1.VolumeSnapshotList
					err := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 10*time.Second, true, func(ctx context.Context) (bool, error) {
						err = pvcBIA.crClient.List(ctx, &vsList, &crclient.ListOptions{Namespace: tc.pvc.Namespace})

						if err != nil || len(vsList.Items) == 0 {
							//lint:ignore nilerr reason
							return false, nil // ignore
						}
						return true, nil
					})

					require.NoError(t, err)
					vscName := "testVSC"
					readyToUse := true
					rsQty := resource.MustParse("1Gi")
					rsValue := rsQty.Value()

					vsList.Items[0].Status = &v1.VolumeSnapshotStatus{
						BoundVolumeSnapshotContentName: &vscName,
						ReadyToUse:                     &readyToUse,
					}
					err = pvcBIA.crClient.Update(context.Background(), &vsList.Items[0])
					require.NoError(t, err)

					handleName := "testHandle"
					vsc := builder.ForVolumeSnapshotContent("testVSC").Status(&snapshotv1api.VolumeSnapshotContentStatus{SnapshotHandle: &handleName, RestoreSize: &rsValue}).Result()
					err = pvcBIA.crClient.Create(context.Background(), vsc)
					require.NoError(t, err)
				}()
			}

			resultUnstructed, resultAdditionalItems, _, _, err := pvcBIA.Execute(&unstructured.Unstructured{Object: pvcMap}, tc.backup)
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
				// Since name is generated, we ignore it in the comparison
				tc.expectedDataUpload.Name = dataUploadList.Items[0].Name
				tc.expectedDataUpload.Spec.CSISnapshot.VolumeSnapshot = dataUploadList.Items[0].Spec.CSISnapshot.VolumeSnapshot
				require.True(t, cmp.Equal(tc.expectedDataUpload, &dataUploadList.Items[0], cmpopts.IgnoreFields(velerov2alpha1.DataUpload{}, "ResourceVersion")))
			}

			if tc.expectedPVC != nil {
				resultPVC := new(corev1api.PersistentVolumeClaim)
				runtime.DefaultUnstructuredConverter.FromUnstructured(resultUnstructed.UnstructuredContent(), resultPVC)
				// Ignore generated names and annotations that may have random suffixes
				require.Contains(t, resultPVC.Annotations[velerov1api.VolumeSnapshotLabel], tc.expectedPVC.Annotations[velerov1api.VolumeSnapshotLabel])
				require.Contains(t, resultPVC.Labels[velerov1api.VolumeSnapshotLabel], tc.expectedPVC.Labels[velerov1api.VolumeSnapshotLabel])
				// check other fields
				require.Equal(t, tc.expectedPVC.Labels[velerov1api.BackupNameLabel], resultPVC.Labels[velerov1api.BackupNameLabel])
			}

			if tc.additionalItems != nil {
				require.Equal(t, len(tc.additionalItems), len(resultAdditionalItems))
				require.Contains(t, resultAdditionalItems[0].Name, tc.additionalItems[0].Name)
				require.Equal(t, tc.additionalItems[0].GroupResource, resultAdditionalItems[0].GroupResource)
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

func TestPVCRequestSize(t *testing.T) {
	logger := logrus.New()

	tests := []struct {
		name         string
		pvcInitial   string // initial storage request on the PVC (e.g. "1Gi" or "3Gi")
		restoreSize  string // restore size set in VSC.Status.RestoreSize (e.g. "2Gi")
		expectedSize string // expected storage request on the PVC after update
		vscStatus    *snapshotv1api.VolumeSnapshotContentStatus
	}{
		{
			name:         "UpdateRequired: PVC request is lower than restore size",
			pvcInitial:   "1Gi",
			restoreSize:  "2Gi",
			expectedSize: "2Gi",
		},
		{
			name:         "NoUpdateRequired: PVC request is larger than restore size",
			pvcInitial:   "3Gi",
			restoreSize:  "2Gi",
			expectedSize: "3Gi",
		},
		// --- NEW TEST CASE ADDED HERE ---
		{
			name:         "NoUpdateRequired: VSC status has nil restore size",
			pvcInitial:   "1Gi",
			restoreSize:  "", // restoreSize is not used when vscStatus is provided
			expectedSize: "1Gi",
			vscStatus: &snapshotv1api.VolumeSnapshotContentStatus{
				RestoreSize: nil, // Explicitly nil
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			crClient := velerotest.NewFakeControllerRuntimeClient(t)

			// Create a PVC with the initial storage request.
			pvc := builder.ForPersistentVolumeClaim("velero", "testPVC").
				VolumeName("testPV").
				StorageClass("testSC").
				Result()
			pvc.Spec.Resources.Requests = corev1api.ResourceList{
				corev1api.ResourceStorage: resource.MustParse(tc.pvcInitial),
			}

			// Create a VolumeSnapshot required for the lookup
			vs := &snapshotv1api.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testVS",
					Namespace: "velero",
				},
			}
			require.NoError(t, crClient.Create(context.Background(), vs))

			// Create a VolumeSnapshotContent with restore size
			vsc := &snapshotv1api.VolumeSnapshotContent{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testVSC",
				},
			}

			if tc.vscStatus != nil {
				vsc.Status = tc.vscStatus
			} else {
				rsQty := resource.MustParse(tc.restoreSize)
				rsValue := rsQty.Value()
				vsc.Status = &snapshotv1api.VolumeSnapshotContentStatus{
					RestoreSize: &rsValue,
				}
			}

			// Call the function under test
			setPVCRequestSizeToVSRestoreSize(pvc, vsc, logger)

			// Verify that the PVC storage request is updated as expected.
			updatedSize := pvc.Spec.Resources.Requests[corev1api.ResourceStorage]
			expected := resource.MustParse(tc.expectedSize)
			require.Equal(t, 0, updatedSize.Cmp(expected), "PVC storage request should be %s", tc.expectedSize)
		})
	}
}
