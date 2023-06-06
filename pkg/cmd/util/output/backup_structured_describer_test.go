package output

import (
	"reflect"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"

	"github.com/vmware-tanzu/velero/pkg/features"
	"github.com/vmware-tanzu/velero/pkg/util/results"

	"github.com/stretchr/testify/assert"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
)

func TestDescribeBackupInSF(t *testing.T) {
	sd := &StructuredDescriber{
		output: make(map[string]interface{}),
		format: "",
	}
	backupBuilder1 := builder.ForBackup("test-ns", "test-backup")
	backupBuilder1.IncludedNamespaces("inc-ns-1", "inc-ns-2").
		ExcludedNamespaces("exc-ns-1", "exc-ns-2").
		IncludedResources("inc-res-1", "inc-res-2").
		ExcludedResources("exc-res-1", "exc-res-2").
		StorageLocation("backup-location").
		TTL(72 * time.Hour).
		CSISnapshotTimeout(10 * time.Minute).
		DataMover("mover")

	expect1 := map[string]interface{}{
		"spec": map[string]interface{}{
			"namespaces": map[string]interface{}{
				"included": "inc-ns-1, inc-ns-2",
				"excluded": "exc-ns-1, exc-ns-2",
			},
			"resources": map[string]string{
				"included":      "inc-res-1, inc-res-2",
				"excluded":      "exc-res-1, exc-res-2",
				"clusterScoped": "auto",
			},
			"dataMover":               "mover",
			"labelSelector":           emptyDisplay,
			"storageLocation":         "backup-location",
			"veleroNativeSnapshotPVs": "auto",
			"TTL":                     "72h0m0s",
			"CSISnapshotTimeout":      "10m0s",
			"veleroSnapshotMoveData":  "auto",
		},
	}
	DescribeBackupSpecInSF(sd, backupBuilder1.Result().Spec)
	assert.True(t, reflect.DeepEqual(sd.output, expect1))

	backupBuilder2 := builder.ForBackup("test-ns-2", "test-backup-2")
	backupBuilder2.StorageLocation("backup-location")
	expect2 := map[string]interface{}{
		"spec": map[string]interface{}{
			"namespaces": map[string]interface{}{
				"included": "*",
				"excluded": emptyDisplay,
			},
			"resources": map[string]string{
				"included":      "*",
				"excluded":      emptyDisplay,
				"clusterScoped": "auto",
			},
			"dataMover":               emptyDisplay,
			"labelSelector":           emptyDisplay,
			"storageLocation":         "backup-location",
			"veleroNativeSnapshotPVs": "auto",
			"TTL":                     "0s",
			"CSISnapshotTimeout":      "0s",
			"veleroSnapshotMoveData":  "auto",
		},
	}
	DescribeBackupSpecInSF(sd, backupBuilder2.Result().Spec)
	assert.True(t, reflect.DeepEqual(sd.output, expect2))
}

func TestDescribePodVolumeBackupsInSF(t *testing.T) {
	pvbBuilder1 := builder.ForPodVolumeBackup("test-ns1", "test-pvb1")
	pvb1 := pvbBuilder1.BackupStorageLocation("backup-location").
		UploaderType("kopia").
		Phase(velerov1api.PodVolumeBackupPhaseCompleted).
		BackupStorageLocation("bsl-1").
		Volume("vol-1").
		PodName("pod-1").
		PodNamespace("pod-ns-1").
		SnapshotID("snap-1").Result()

	pvbBuilder2 := builder.ForPodVolumeBackup("test-ns1", "test-pvb2")
	pvb2 := pvbBuilder2.BackupStorageLocation("backup-location").
		UploaderType("kopia").
		Phase(velerov1api.PodVolumeBackupPhaseCompleted).
		BackupStorageLocation("bsl-1").
		Volume("vol-2").
		PodName("pod-2").
		PodNamespace("pod-ns-1").
		SnapshotID("snap-2").Result()

	testcases := []struct {
		name         string
		inputPVBList []velerov1api.PodVolumeBackup
		inputDetails bool
		expect       map[string]interface{}
	}{
		{
			name:         "empty list",
			inputPVBList: []velerov1api.PodVolumeBackup{},
			inputDetails: false,
			expect:       map[string]interface{}{},
		},
		{
			name:         "2 completed pvbs",
			inputPVBList: []velerov1api.PodVolumeBackup{*pvb1, *pvb2},
			inputDetails: true,
			expect: map[string]interface{}{
				"podVolumeBackups": map[string]interface{}{
					"podVolumeBackupsDetails": map[string]interface{}{
						"Completed": []map[string]string{
							{"pod-ns-1/pod-1": "vol-1"},
							{"pod-ns-1/pod-2": "vol-2"},
						},
					},
					"type": "kopia",
				},
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(tt *testing.T) {
			sd := &StructuredDescriber{
				output: make(map[string]interface{}),
				format: "",
			}
			DescribePodVolumeBackupsInSF(sd, tc.inputPVBList, tc.inputDetails)
			assert.True(tt, reflect.DeepEqual(sd.output, tc.expect))
		})
	}
}

func TestDescribeCSIVolumeSnapshotsInSF(t *testing.T) {
	features.Enable(velerov1api.CSIFeatureFlag)
	defer func() {
		features.Disable(velerov1api.CSIFeatureFlag)
	}()

	vscBuilder1 := builder.ForVolumeSnapshotContent("vsc-1")
	handle := "handle-1"
	readyToUse := true
	size := int64(1024)
	vsc1 := vscBuilder1.Status(&snapshotv1api.VolumeSnapshotContentStatus{
		SnapshotHandle: &handle,
		ReadyToUse:     &readyToUse,
		RestoreSize:    &size,
	}).Result()

	testcases := []struct {
		name         string
		inputVSCList []snapshotv1api.VolumeSnapshotContent
		inputDetails bool
		expect       map[string]interface{}
	}{
		{
			name:         "empty list",
			inputVSCList: []snapshotv1api.VolumeSnapshotContent{},
			inputDetails: false,
			expect:       map[string]interface{}{},
		},
		{
			name:         "1 vsc no detail",
			inputVSCList: []snapshotv1api.VolumeSnapshotContent{*vsc1},
			inputDetails: false,
			expect: map[string]interface{}{
				"CSIVolumeSnapshots": map[string]interface{}{
					"CSIVolumeSnapshotsCount": 1,
				},
			},
		},
		{
			name:         "1 vsc with detail",
			inputVSCList: []snapshotv1api.VolumeSnapshotContent{*vsc1},
			inputDetails: true,
			expect: map[string]interface{}{
				"CSIVolumeSnapshots": map[string]interface{}{
					"CSIVolumeSnapshotsDetails": map[string]interface{}{
						"vsc-1": map[string]interface{}{
							"readyToUse":          true,
							"snapshotSize(bytes)": int64(1024),
							"storageSnapshotID":   "handle-1",
						},
					},
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(tt *testing.T) {
			sd := &StructuredDescriber{
				output: make(map[string]interface{}),
				format: "",
			}
			DescribeCSIVolumeSnapshotsInSF(sd, tc.inputDetails, tc.inputVSCList)
			assert.True(tt, reflect.DeepEqual(sd.output, tc.expect))
		})
	}
}

func TestDescribeResourcePoliciesInSF(t *testing.T) {
	input := &v1.TypedLocalObjectReference{
		Kind: "configmap",
		Name: "resource-policy-1",
	}
	expect := map[string]interface{}{
		"resourcePolicies": map[string]interface{}{
			"type": "configmap",
			"name": "resource-policy-1",
		},
	}
	sd := &StructuredDescriber{
		output: make(map[string]interface{}),
		format: "",
	}
	DescribeResourcePoliciesInSF(sd, input)
	assert.True(t, reflect.DeepEqual(sd.output, expect))
}

func TestDescribeBackupResultInSF(t *testing.T) {
	input := results.Result{
		Velero:  []string{"msg-1", "msg-2"},
		Cluster: []string{"cluster-1", "cluster-2"},
		Namespaces: map[string][]string{
			"ns-1": {"ns-1-msg-1", "ns-1-msg-2"},
		},
	}
	got := map[string]interface{}{}
	expect := map[string]interface{}{
		"velero":  []string{"msg-1", "msg-2"},
		"cluster": []string{"cluster-1", "cluster-2"},
		"namespace": map[string][]string{
			"ns-1": {"ns-1-msg-1", "ns-1-msg-2"},
		},
	}
	describeResultInSF(got, input)
	assert.True(t, reflect.DeepEqual(got, expect))
}
