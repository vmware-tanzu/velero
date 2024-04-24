package volumehelper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"

	"github.com/vmware-tanzu/velero/internal/resourcepolicies"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestVolumeHelperImpl_ShouldPerformSnapshot(t *testing.T) {
	PVObjectGP2 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "PersistentVolume",
			"metadata": map[string]interface{}{
				"name": "example-pv",
			},
			"spec": map[string]interface{}{
				"capacity": map[string]interface{}{
					"storage": "1Gi",
				},
				"volumeMode":                    "Filesystem",
				"accessModes":                   []interface{}{"ReadWriteOnce"},
				"persistentVolumeReclaimPolicy": "Retain",
				"storageClassName":              "gp2-csi",
				"hostPath": map[string]interface{}{
					"path": "/mnt/data",
				},
			},
		},
	}

	PVObjectGP3 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "PersistentVolume",
			"metadata": map[string]interface{}{
				"name": "example-pv",
			},
			"spec": map[string]interface{}{
				"capacity": map[string]interface{}{
					"storage": "1Gi",
				},
				"volumeMode":                    "Filesystem",
				"accessModes":                   []interface{}{"ReadWriteOnce"},
				"persistentVolumeReclaimPolicy": "Retain",
				"storageClassName":              "gp3-csi",
				"hostPath": map[string]interface{}{
					"path": "/mnt/data",
				},
			},
		},
	}

	testCases := []struct {
		name                string
		obj                 runtime.Unstructured
		groupResource       schema.GroupResource
		resourcePolicies    resourcepolicies.ResourcePolicies
		snapshotVolumesFlag *bool
		shouldSnapshot      bool
		expectedErr         bool
	}{
		{
			name:          "Given PV object matches volume policy snapshot action snapshotVolumes flags is true returns true and no error",
			obj:           PVObjectGP2,
			groupResource: kuberesource.PersistentVolumes,
			resourcePolicies: resourcepolicies.ResourcePolicies{
				Version: "v1",
				VolumePolicies: []resourcepolicies.VolumePolicy{
					{
						Conditions: map[string]interface{}{
							"storageClass": []string{"gp2-csi"},
						},
						Action: resourcepolicies.Action{
							Type: resourcepolicies.Snapshot,
						},
					},
				},
			},
			snapshotVolumesFlag: pointer.Bool(true),
			shouldSnapshot:      true,
			expectedErr:         false,
		},
		{
			name:          "Given PV object matches volume policy snapshot action snapshotVolumes flags is false returns false and no error",
			obj:           PVObjectGP2,
			groupResource: kuberesource.PersistentVolumes,
			resourcePolicies: resourcepolicies.ResourcePolicies{
				Version: "v1",
				VolumePolicies: []resourcepolicies.VolumePolicy{
					{
						Conditions: map[string]interface{}{
							"storageClass": []string{"gp2-csi"},
						},
						Action: resourcepolicies.Action{
							Type: resourcepolicies.Snapshot,
						},
					},
				},
			},
			snapshotVolumesFlag: pointer.Bool(false),
			shouldSnapshot:      false,
			expectedErr:         false,
		},
		{
			name:          "Given PV object matches volume policy snapshot action snapshotVolumes flags is true returns false and no error",
			obj:           PVObjectGP3,
			groupResource: kuberesource.PersistentVolumes,
			resourcePolicies: resourcepolicies.ResourcePolicies{
				Version: "v1",
				VolumePolicies: []resourcepolicies.VolumePolicy{
					{
						Conditions: map[string]interface{}{
							"storageClass": []string{"gp2-csi"},
						},
						Action: resourcepolicies.Action{
							Type: resourcepolicies.Snapshot,
						},
					},
				},
			},
			snapshotVolumesFlag: pointer.Bool(true),
			shouldSnapshot:      false,
			expectedErr:         false,
		},
		{
			name: "Given PVC object matches volume policy snapshot action snapshotVolumes flags is true return false and error case PVC not found",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "PersistentVolumeClaim",
					"metadata": map[string]interface{}{
						"name":      "example-pvc",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"accessModes": []string{"ReadWriteOnce"},
						"resources": map[string]interface{}{
							"requests": map[string]interface{}{
								"storage": "1Gi",
							},
						},
						"storageClassName": "gp2-csi",
					},
				},
			},
			groupResource: kuberesource.PersistentVolumeClaims,
			resourcePolicies: resourcepolicies.ResourcePolicies{
				Version: "v1",
				VolumePolicies: []resourcepolicies.VolumePolicy{
					{
						Conditions: map[string]interface{}{
							"storageClass": []string{"gp2-csi"},
						},
						Action: resourcepolicies.Action{
							Type: resourcepolicies.Snapshot,
						},
					},
				},
			},
			snapshotVolumesFlag: pointer.Bool(true),
			shouldSnapshot:      false,
			expectedErr:         true,
		},
	}

	mockedPV := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "example-pv",
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("1Gi"),
			},
			AccessModes:                   []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			StorageClassName:              "gp2-csi",
			ClaimRef: &corev1.ObjectReference{
				Name:      "example-pvc",
				Namespace: "default",
			},
		},
	}

	objs := []runtime.Object{mockedPV}
	fakeClient := velerotest.NewFakeControllerRuntimeClient(t, objs...)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			policies := tc.resourcePolicies
			p := &resourcepolicies.Policies{}
			err := p.BuildPolicy(&policies)
			if err != nil {
				t.Fatalf("failed to build policy with error %v", err)
			}
			vh := &VolumeHelperImpl{
				VolumePolicy:    p,
				SnapshotVolumes: tc.snapshotVolumesFlag,
				Logger:          velerotest.NewLogger(),
			}
			ActualShouldSnapshot, actualError := vh.ShouldPerformSnapshot(tc.obj, tc.groupResource, fakeClient)
			if tc.expectedErr {
				assert.NotNil(t, actualError, "Want error; Got nil error")
				return
			}

			assert.Equalf(t, ActualShouldSnapshot, tc.shouldSnapshot, "Want shouldSnapshot as %v; Got shouldSnapshot as %v", tc.shouldSnapshot, ActualShouldSnapshot)
		})
	}
}

func TestVolumeHelperImpl_ShouldIncludeVolumeInBackup(t *testing.T) {
	testCases := []struct {
		name             string
		vol              corev1.Volume
		backupExcludePVC bool
		shouldInclude    bool
	}{
		{
			name: "volume has host path so do not include",
			vol: corev1.Volume{
				Name: "sample-volume",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "some-path",
					},
				},
			},
			backupExcludePVC: false,
			shouldInclude:    false,
		},
		{
			name: "volume has secret mounted so do not include",
			vol: corev1.Volume{
				Name: "sample-volume",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "sample-secret",
						Items: []corev1.KeyToPath{
							{
								Key:  "username",
								Path: "my-username",
							},
						},
					},
				},
			},
			backupExcludePVC: false,
			shouldInclude:    false,
		},
		{
			name: "volume has configmap so do not include",
			vol: corev1.Volume{
				Name: "sample-volume",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "sample-cm",
						},
					},
				},
			},
			backupExcludePVC: false,
			shouldInclude:    false,
		},
		{
			name: "volume is mounted as project volume so do not include",
			vol: corev1.Volume{
				Name: "sample-volume",
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{},
					},
				},
			},
			backupExcludePVC: false,
			shouldInclude:    false,
		},
		{
			name: "volume has downwardAPI so do not include",
			vol: corev1.Volume{
				Name: "sample-volume",
				VolumeSource: corev1.VolumeSource{
					DownwardAPI: &corev1.DownwardAPIVolumeSource{
						Items: []corev1.DownwardAPIVolumeFile{
							{
								Path: "labels",
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: "metadata.labels",
								},
							},
						},
					},
				},
			},
			backupExcludePVC: false,
			shouldInclude:    false,
		},
		{
			name: "volume has pvc and backupExcludePVC is true so do not include",
			vol: corev1.Volume{
				Name: "sample-volume",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "sample-pvc",
					},
				},
			},
			backupExcludePVC: true,
			shouldInclude:    false,
		},
		{
			name: "volume name has prefix default-token so do not include",
			vol: corev1.Volume{
				Name: "default-token-vol-name",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "sample-pvc",
					},
				},
			},
			backupExcludePVC: false,
			shouldInclude:    false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resourcePolicies := resourcepolicies.ResourcePolicies{
				Version: "v1",
				VolumePolicies: []resourcepolicies.VolumePolicy{
					{
						Conditions: map[string]interface{}{
							"storageClass": []string{"gp2-csi"},
						},
						Action: resourcepolicies.Action{
							Type: resourcepolicies.Snapshot,
						},
					},
				},
			}
			policies := resourcePolicies
			p := &resourcepolicies.Policies{}
			err := p.BuildPolicy(&policies)
			if err != nil {
				t.Fatalf("failed to build policy with error %v", err)
			}
			vh := &VolumeHelperImpl{
				VolumePolicy:    p,
				SnapshotVolumes: pointer.Bool(true),
				Logger:          velerotest.NewLogger(),
			}
			actualShouldInclude := vh.ShouldIncludeVolumeInBackup(tc.vol, tc.backupExcludePVC)
			assert.Equalf(t, actualShouldInclude, tc.shouldInclude, "Want shouldInclude as %v; Got actualShouldInclude as %v", tc.shouldInclude, actualShouldInclude)
		})
	}
}

var (
	gp2csi = "gp2-csi"
	gp3csi = "gp3-csi"
)
var (
	samplePod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample-pod",
			Namespace: "sample-ns",
		},

		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "sample-container",
					Image: "sample-image",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "sample-vm",
							MountPath: "/etc/pod-info",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "sample-volume-1",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "sample-pvc-1",
						},
					},
				},
				{
					Name: "sample-volume-2",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "sample-pvc-2",
						},
					},
				},
			},
		},
	}

	samplePVC1 = &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample-pvc-1",
			Namespace: "sample-ns",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{},
			},
			StorageClassName: &gp2csi,
			VolumeName:       "sample-pv-1",
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase:       corev1.ClaimBound,
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Capacity:    corev1.ResourceList{},
		},
	}

	samplePVC2 = &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample-pvc-2",
			Namespace: "sample-ns",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{},
			},
			StorageClassName: &gp3csi,
			VolumeName:       "sample-pv-2",
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase:       corev1.ClaimBound,
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Capacity:    corev1.ResourceList{},
		},
	}

	samplePV1 = &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sample-pv-1",
		},
		Spec: corev1.PersistentVolumeSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Capacity:    corev1.ResourceList{},
			ClaimRef: &corev1.ObjectReference{
				Kind:            "PersistentVolumeClaim",
				Name:            "sample-pvc-1",
				Namespace:       "sample-ns",
				ResourceVersion: "1027",
				UID:             "7d28e566-ade7-4ed6-9e15-2e44d2fbcc08",
			},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver: "ebs.csi.aws.com",
					FSType: "ext4",
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": "1582049697841-8081-hostpath.csi.k8s.io",
					},
					VolumeHandle: "e61f2b48-527a-11ea-b54f-cab6317018f1",
				},
			},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
			StorageClassName:              gp2csi,
		},
		Status: corev1.PersistentVolumeStatus{
			Phase: corev1.VolumeBound,
		},
	}

	samplePV2 = &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sample-pv-2",
		},
		Spec: corev1.PersistentVolumeSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Capacity:    corev1.ResourceList{},
			ClaimRef: &corev1.ObjectReference{
				Kind:            "PersistentVolumeClaim",
				Name:            "sample-pvc-2",
				Namespace:       "sample-ns",
				ResourceVersion: "1027",
				UID:             "7d28e566-ade7-4ed6-9e15-2e44d2fbcc08",
			},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver: "ebs.csi.aws.com",
					FSType: "ext4",
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": "1582049697841-8081-hostpath.csi.k8s.io",
					},
					VolumeHandle: "e61f2b48-527a-11ea-b54f-cab6317018f1",
				},
			},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
			StorageClassName:              gp3csi,
		},
		Status: corev1.PersistentVolumeStatus{
			Phase: corev1.VolumeBound,
		},
	}
	resourcePolicies1 = resourcepolicies.ResourcePolicies{
		Version: "v1",
		VolumePolicies: []resourcepolicies.VolumePolicy{
			{
				Conditions: map[string]interface{}{
					"storageClass": []string{"gp2-csi"},
				},
				Action: resourcepolicies.Action{
					Type: resourcepolicies.FSBackup,
				},
			},
			{
				Conditions: map[string]interface{}{
					"storageClass": []string{"gp3-csi"},
				},
				Action: resourcepolicies.Action{
					Type: resourcepolicies.Snapshot,
				},
			},
		},
	}

	resourcePolicies2 = resourcepolicies.ResourcePolicies{
		Version: "v1",
		VolumePolicies: []resourcepolicies.VolumePolicy{
			{
				Conditions: map[string]interface{}{
					"storageClass": []string{"gp2-csi"},
				},
				Action: resourcepolicies.Action{
					Type: resourcepolicies.FSBackup,
				},
			},
		},
	}

	resourcePolicies3 = resourcepolicies.ResourcePolicies{
		Version: "v1",
		VolumePolicies: []resourcepolicies.VolumePolicy{
			{
				Conditions: map[string]interface{}{
					"storageClass": []string{"gp4-csi"},
				},
				Action: resourcepolicies.Action{
					Type: resourcepolicies.FSBackup,
				},
			},
		},
	}
)

func TestVolumeHelperImpl_GetVolumesMatchingFSBackupAction(t *testing.T) {
	testCases := []struct {
		name                          string
		backupExcludePVC              bool
		resourcepoliciesApplied       resourcepolicies.ResourcePolicies
		FSBackupActionMatchingVols    []string
		FSBackupNonActionMatchingVols []string
		NoActionMatchingVols          []string
		expectedErr                   bool
	}{
		{
			name:                          "For a given pod with 2 volumes and volume policy we get one fs-backup action matching volume, one fs-back action non-matching volume but has some matching action and zero no action matching volumes",
			backupExcludePVC:              false,
			resourcepoliciesApplied:       resourcePolicies1,
			FSBackupActionMatchingVols:    []string{"sample-volume-1"},
			FSBackupNonActionMatchingVols: []string{"sample-volume-2"},
			NoActionMatchingVols:          []string{},
			expectedErr:                   false,
		},
		{
			name:                          "For a given pod with 2 volumes and volume policy we get one fs-backup action matching volume, zero fs-backup action non-matching volume and one no action matching volumes",
			backupExcludePVC:              false,
			resourcepoliciesApplied:       resourcePolicies2,
			FSBackupActionMatchingVols:    []string{"sample-volume-1"},
			FSBackupNonActionMatchingVols: []string{},
			NoActionMatchingVols:          []string{"sample-volume-2"},
			expectedErr:                   false,
		},
		{
			name:                          "For a given pod with 2 volumes and volume policy we get one fs-backup action matching volume, one fs-backup action non-matching volume and one no action matching volumes but backupExcludePVC is true so all returned list should be empty",
			backupExcludePVC:              true,
			resourcepoliciesApplied:       resourcePolicies2,
			FSBackupActionMatchingVols:    []string{},
			FSBackupNonActionMatchingVols: []string{},
			NoActionMatchingVols:          []string{},
			expectedErr:                   false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			policies := tc.resourcepoliciesApplied
			p := &resourcepolicies.Policies{}
			err := p.BuildPolicy(&policies)
			if err != nil {
				t.Fatalf("failed to build policy with error %v", err)
			}
			vh := &VolumeHelperImpl{
				VolumePolicy:    p,
				SnapshotVolumes: pointer.Bool(true),
				Logger:          velerotest.NewLogger(),
			}
			objs := []runtime.Object{samplePod, samplePVC1, samplePVC2, samplePV1, samplePV2}
			fakeClient := velerotest.NewFakeControllerRuntimeClient(t, objs...)
			gotFSBackupActionMatchingVols, gotFSBackupNonActionMatchingVols, gotNoActionMatchingVols, actualError := vh.GetVolumesMatchingFSBackupAction(samplePod, vh.VolumePolicy, tc.backupExcludePVC, fakeClient)
			if tc.expectedErr {
				assert.NotNil(t, actualError, "Want error; Got nil error")
				return
			}
			assert.Nilf(t, actualError, "Want: nil error; Got: %v", actualError)
			assert.Equalf(t, gotFSBackupActionMatchingVols, tc.FSBackupActionMatchingVols, "Want FSBackupActionMatchingVols as %v; Got gotFSBackupActionMatchingVols as %v", tc.FSBackupActionMatchingVols, gotFSBackupActionMatchingVols)
			assert.Equalf(t, gotFSBackupNonActionMatchingVols, tc.FSBackupNonActionMatchingVols, "Want FSBackupNonActionMatchingVols as %v; Got gotFSBackupNonActionMatchingVols as %v", tc.FSBackupNonActionMatchingVols, gotFSBackupNonActionMatchingVols)
			assert.Equalf(t, gotNoActionMatchingVols, tc.NoActionMatchingVols, "Want NoActionMatchingVols as %v; Got gotNoActionMatchingVols as %v", tc.NoActionMatchingVols, gotNoActionMatchingVols)
		})
	}
}

func TestVolumeHelperImpl_GetVolumesForFSBackup(t *testing.T) {
	testCases := []struct {
		name                     string
		backupExcludePVC         bool
		defaultVolumesToFsBackup bool
		resourcepoliciesApplied  resourcepolicies.ResourcePolicies
		includedVolumes          []string
		optedOutVolumes          []string
		expectedErr              bool
	}{
		{
			name:                     "For a given pod with 2 volumes and volume policy we get one fs-backup action matching volume, one fs-back action non-matching volume but matches snapshot action so no volumes for legacy fallback process, defaultvolumestofsbackup is false but no effect",
			backupExcludePVC:         false,
			defaultVolumesToFsBackup: false,
			resourcepoliciesApplied:  resourcePolicies1,
			includedVolumes:          []string{"sample-volume-1"},
			optedOutVolumes:          []string{"sample-volume-2"},
		},
		{
			name:                     "For a given pod with 2 volumes and volume policy we get one fs-backup action matching volume, one fs-back action non-matching volume but matches snapshot action so no volumes for legacy fallback process, defaultvolumestofsbackup is true but no effect",
			backupExcludePVC:         false,
			defaultVolumesToFsBackup: true,
			resourcepoliciesApplied:  resourcePolicies1,
			includedVolumes:          []string{"sample-volume-1"},
			optedOutVolumes:          []string{"sample-volume-2"},
		},
		{
			name:                     "For a given pod with 2 volumes and volume policy we get no volume matching fs-backup action defaultvolumesToFSBackup is false, no annotations, using legacy as fallback for non-action matching volumes",
			backupExcludePVC:         false,
			defaultVolumesToFsBackup: false,
			resourcepoliciesApplied:  resourcePolicies3,
			includedVolumes:          []string{},
			optedOutVolumes:          []string{},
		},
		{
			name:                     "For a given pod with 2 volumes and volume policy we get no volume matching fs-backup action defaultvolumesToFSBackup is true, no annotations, using legacy as fallback for non-action matching volumes",
			backupExcludePVC:         false,
			defaultVolumesToFsBackup: true,
			resourcepoliciesApplied:  resourcePolicies3,
			includedVolumes:          []string{"sample-volume-1", "sample-volume-2"},
			optedOutVolumes:          []string{},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			policies := tc.resourcepoliciesApplied
			p := &resourcepolicies.Policies{}
			err := p.BuildPolicy(&policies)
			if err != nil {
				t.Fatalf("failed to build policy with error %v", err)
			}
			vh := &VolumeHelperImpl{
				VolumePolicy:    p,
				SnapshotVolumes: pointer.Bool(true),
				Logger:          velerotest.NewLogger(),
			}
			objs := []runtime.Object{samplePod, samplePVC1, samplePVC2, samplePV1, samplePV2}
			fakeClient := velerotest.NewFakeControllerRuntimeClient(t, objs...)
			gotIncludedVolumes, gotOptedOutVolumes, actualError := vh.GetVolumesForFSBackup(samplePod, tc.defaultVolumesToFsBackup, tc.backupExcludePVC, fakeClient)
			if tc.expectedErr {
				assert.NotNil(t, actualError, "Want error; Got nil error")
				return
			}
			assert.Nilf(t, actualError, "Want: nil error; Got: %v", actualError)
			assert.Equalf(t, tc.includedVolumes, gotIncludedVolumes, "Want includedVolumes as %v; Got gotIncludedVolumes as %v", tc.includedVolumes, gotIncludedVolumes)
			assert.Equalf(t, tc.optedOutVolumes, gotOptedOutVolumes, "Want optedOutVolumes as %v; Got gotOptedOutVolumes as %v", tc.optedOutVolumes, gotOptedOutVolumes)
		})
	}
}
