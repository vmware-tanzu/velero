package output

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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
		DataMover("mover").
		Hooks(velerov1api.BackupHooks{
			Resources: []velerov1api.BackupResourceHookSpec{
				{
					Name: "hook-1",
					PreHooks: []velerov1api.BackupResourceHook{
						{
							Exec: &velerov1api.ExecHook{
								Container: "hook-container-1",
								Command:   []string{"pre"},
								OnError:   velerov1api.HookErrorModeContinue,
							},
						},
					},
					PostHooks: []velerov1api.BackupResourceHook{
						{
							Exec: &velerov1api.ExecHook{
								Container: "hook-container-1",
								Command:   []string{"post"},
								OnError:   velerov1api.HookErrorModeContinue,
							},
						},
					},
					IncludedNamespaces: []string{"hook-inc-ns-1", "hook-inc-ns-2"},
					ExcludedNamespaces: []string{"hook-exc-ns-1", "hook-exc-ns-2"},
					IncludedResources:  []string{"hook-inc-res-1", "hook-inc-res-2"},
					ExcludedResources:  []string{"hook-exc-res-1", "hook-exc-res-2"},
				},
			},
		})

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
			"hooks": map[string]interface{}{
				"resources": map[string]interface{}{
					"hook-1": map[string]interface{}{
						"labelSelector": emptyDisplay,
						"namespaces": map[string]string{
							"included": "hook-inc-ns-1, hook-inc-ns-2",
							"excluded": "hook-exc-ns-1, hook-exc-ns-2",
						},
						"preExecHook": []map[string]interface{}{
							{
								"container": "hook-container-1",
								"command":   "pre",
								"onError:":  velerov1api.HookErrorModeContinue,
								"timeout":   "0s",
							},
						},
						"postExecHook": []map[string]interface{}{
							{
								"container": "hook-container-1",
								"command":   "post",
								"onError:":  velerov1api.HookErrorModeContinue,
								"timeout":   "0s",
							},
						},
						"resources": map[string]string{
							"included": "hook-inc-res-1, hook-inc-res-2",
							"excluded": "hook-exc-res-1, hook-exc-res-2",
						},
					},
				},
			},
		},
	}
	DescribeBackupSpecInSF(sd, backupBuilder1.Result().Spec)
	assert.True(t, reflect.DeepEqual(sd.output, expect1))

	backupBuilder2 := builder.ForBackup("test-ns-2", "test-backup-2").
		StorageLocation("backup-location").
		OrderedResources(map[string]string{
			"kind1": "rs1-1, rs1-2",
			"kind2": "rs2-1, rs2-2",
		}).Hooks(velerov1api.BackupHooks{
		Resources: []velerov1api.BackupResourceHookSpec{
			{
				Name: "hook-1",
				PreHooks: []velerov1api.BackupResourceHook{
					{
						Exec: &velerov1api.ExecHook{
							Container: "hook-container-1",
							Command:   []string{"pre"},
							OnError:   velerov1api.HookErrorModeContinue,
						},
					},
				},
				PostHooks: []velerov1api.BackupResourceHook{
					{
						Exec: &velerov1api.ExecHook{
							Container: "hook-container-1",
							Command:   []string{"post"},
							OnError:   velerov1api.HookErrorModeContinue,
						},
					},
				},
			},
		},
	})

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
			"hooks": map[string]interface{}{
				"resources": map[string]interface{}{
					"hook-1": map[string]interface{}{
						"labelSelector": emptyDisplay,
						"namespaces": map[string]string{
							"included": "*",
							"excluded": emptyDisplay,
						},
						"preExecHook": []map[string]interface{}{
							{
								"container": "hook-container-1",
								"command":   "pre",
								"onError:":  velerov1api.HookErrorModeContinue,
								"timeout":   "0s",
							},
						},
						"postExecHook": []map[string]interface{}{
							{
								"container": "hook-container-1",
								"command":   "post",
								"onError:":  velerov1api.HookErrorModeContinue,
								"timeout":   "0s",
							},
						},
						"resources": map[string]string{
							"included": "*",
							"excluded": emptyDisplay,
						},
					},
				},
			},
			"orderedResources": map[string]string{
				"kind1": "rs1-1, rs1-2",
				"kind2": "rs2-1, rs2-2",
			},
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

func TestDescribeDeleteBackupRequestsInSF(t *testing.T) {
	t1, err1 := time.Parse("2006-Jan-02", "2023-Jun-26")
	require.Nil(t, err1)
	dbr1 := builder.ForDeleteBackupRequest("velero", "dbr1").
		ObjectMeta(builder.WithCreationTimestamp(t1)).
		BackupName("bak-1").
		Phase(velerov1api.DeleteBackupRequestPhaseProcessed).
		Errors("some error").Result()
	t2, err2 := time.Parse("2006-Jan-02", "2023-Jun-25")
	require.Nil(t, err2)
	dbr2 := builder.ForDeleteBackupRequest("velero", "dbr2").
		ObjectMeta(builder.WithCreationTimestamp(t2)).
		BackupName("bak-2").
		Phase(velerov1api.DeleteBackupRequestPhaseInProgress).Result()

	testcases := []struct {
		name   string
		input  []velerov1api.DeleteBackupRequest
		expect map[string]interface{}
	}{
		{
			name:  "empty list",
			input: []velerov1api.DeleteBackupRequest{},
			expect: map[string]interface{}{
				"deletionAttempts": map[string]interface{}{
					"deleteBackupRequests": []map[string]interface{}{},
				},
			},
		},
		{
			name:  "list with one failed and one in-progress request",
			input: []velerov1api.DeleteBackupRequest{*dbr1, *dbr2},
			expect: map[string]interface{}{
				"deletionAttempts": map[string]interface{}{
					"failed": int(1),
					"deleteBackupRequests": []map[string]interface{}{
						{
							"creationTimestamp": t1.String(),
							"phase":             velerov1api.DeleteBackupRequestPhaseProcessed,
							"errors": []string{
								"some error",
							},
						},
						{
							"creationTimestamp": t2.String(),
							"phase":             velerov1api.DeleteBackupRequestPhaseInProgress,
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
			DescribeDeleteBackupRequestsInSF(sd, tc.input)
			assert.True(tt, reflect.DeepEqual(sd.output, tc.expect))
		})
	}

}

func TestDescribeSnapshotInSF(t *testing.T) {
	res := map[string]interface{}{}
	iops := int64(100)
	describeSnapshotInSF("pv-1", "snapshot-1", "ebs", "us-east-2", &iops, res)
	expect := map[string]interface{}{
		"pv-1": map[string]string{
			"snapshotID":       "snapshot-1",
			"type":             "ebs",
			"availabilityZone": "us-east-2",
			"IOPS":             "100",
		},
	}
	assert.True(t, reflect.DeepEqual(expect, res))
}
