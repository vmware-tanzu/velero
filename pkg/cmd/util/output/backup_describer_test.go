package output

import (
	"bytes"
	"testing"
	"text/tabwriter"
	"time"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"

	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/features"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

func TestDescribeResourcePolicies(t *testing.T) {
	input := &v1.TypedLocalObjectReference{
		Kind: "configmap",
		Name: "test-resource-policy",
	}
	d := &Describer{
		Prefix: "",
		out:    &tabwriter.Writer{},
		buf:    &bytes.Buffer{},
	}
	d.out.Init(d.buf, 0, 8, 2, ' ', 0)
	DescribeResourcePolicies(d, input)
	d.out.Flush()
	expect := `Resource policies:
  Type:  configmap
  Name:  test-resource-policy
`
	assert.Equal(t, expect, d.buf.String())
}

func TestDescribeBackupSpec(t *testing.T) {
	input1 := builder.ForBackup("test-ns", "test-backup-1").
		IncludedNamespaces("inc-ns-1", "inc-ns-2").
		ExcludedNamespaces("exc-ns-1", "exc-ns-2").
		IncludedResources("inc-res-1", "inc-res-2").
		ExcludedResources("exc-res-1", "exc-res-2").
		StorageLocation("backup-location").
		TTL(72 * time.Hour).
		CSISnapshotTimeout(10 * time.Minute).
		DataMover("mover").
		Result().Spec

	expect1 := `Namespaces:
  Included:  inc-ns-1, inc-ns-2
  Excluded:  exc-ns-1, exc-ns-2

Resources:
  Included:        inc-res-1, inc-res-2
  Excluded:        exc-res-1, exc-res-2
  Cluster-scoped:  auto

Label selector:  <none>

Storage Location:  backup-location

Velero-Native Snapshot PVs:  auto
Snapshot Move Data:          auto
Data Mover:                  mover

TTL:  72h0m0s

CSISnapshotTimeout:    10m0s
ItemOperationTimeout:  0s

Hooks:  <none>
`

	input2 := builder.ForBackup("test-ns", "test-backup-2").
		IncludedNamespaces("inc-ns-1", "inc-ns-2").
		ExcludedNamespaces("exc-ns-1", "exc-ns-2").
		IncludedClusterScopedResources("inc-cluster-res-1", "inc-cluster-res-2").
		IncludedNamespaceScopedResources("inc-ns-res-1", "inc-ns-res-2").
		ExcludedClusterScopedResources("exc-cluster-res-1", "exc-cluster-res-2").
		ExcludedNamespaceScopedResources("exc-ns-res-1", "exc-ns-res-2").
		StorageLocation("backup-location").
		TTL(72 * time.Hour).
		CSISnapshotTimeout(10 * time.Minute).
		DataMover("mover").
		Result().Spec

	expect2 := `Namespaces:
  Included:  inc-ns-1, inc-ns-2
  Excluded:  exc-ns-1, exc-ns-2

Resources:
  Included cluster-scoped:    inc-cluster-res-1, inc-cluster-res-2
  Excluded cluster-scoped:    exc-cluster-res-1, exc-cluster-res-2
  Included namespace-scoped:  inc-ns-res-1, inc-ns-res-2
  Excluded namespace-scoped:  exc-ns-res-1, exc-ns-res-2

Label selector:  <none>

Storage Location:  backup-location

Velero-Native Snapshot PVs:  auto
Snapshot Move Data:          auto
Data Mover:                  mover

TTL:  72h0m0s

CSISnapshotTimeout:    10m0s
ItemOperationTimeout:  0s

Hooks:  <none>
`

	testcases := []struct {
		name   string
		input  velerov1api.BackupSpec
		expect string
	}{
		{
			name:   "old resource filter",
			input:  input1,
			expect: expect1,
		},
		{
			name:   "new resource filter",
			input:  input2,
			expect: expect2,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(tt *testing.T) {
			d := &Describer{
				Prefix: "",
				out:    &tabwriter.Writer{},
				buf:    &bytes.Buffer{},
			}
			d.out.Init(d.buf, 0, 8, 2, ' ', 0)
			DescribeBackupSpec(d, tc.input)
			d.out.Flush()
			assert.Equal(tt, tc.expect, d.buf.String())
		})
	}
}

func TestDescribeSnapshot(t *testing.T) {
	d := &Describer{
		Prefix: "",
		out:    &tabwriter.Writer{},
		buf:    &bytes.Buffer{},
	}
	d.out.Init(d.buf, 0, 8, 2, ' ', 0)
	describeSnapshot(d, "pv-1", "snapshot-1", "ebs", "us-east-2", nil)
	expect1 := `  pv-1:
    Snapshot ID:        snapshot-1
    Type:               ebs
    Availability Zone:  us-east-2
    IOPS:               <N/A>
`
	d.out.Flush()
	assert.Equal(t, expect1, d.buf.String())
}

func TestDescribePodVolumeBackups(t *testing.T) {
	pvb1 := builder.ForPodVolumeBackup("test-ns", "test-pvb1").
		BackupStorageLocation("backup-location").
		UploaderType("kopia").
		Phase(velerov1api.PodVolumeBackupPhaseCompleted).
		BackupStorageLocation("bsl-1").
		Volume("vol-1").
		PodName("pod-1").
		PodNamespace("pod-ns-1").
		SnapshotID("snap-1").Result()
	pvb2 := builder.ForPodVolumeBackup("test-ns1", "test-pvb2").
		BackupStorageLocation("backup-location").
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
		expect       string
	}{
		{
			name:         "empty list",
			inputPVBList: []velerov1api.PodVolumeBackup{},
			inputDetails: true,
			expect:       ``,
		},
		{
			name:         "2 completed pvbs no details",
			inputPVBList: []velerov1api.PodVolumeBackup{*pvb1, *pvb2},
			inputDetails: false,
			expect: `kopia Backups (specify --details for more information):
  Completed:  2
`,
		},
		{
			name:         "2 completed pvbs with details",
			inputPVBList: []velerov1api.PodVolumeBackup{*pvb1, *pvb2},
			inputDetails: true,
			expect: `kopia Backups:
  Completed:
    pod-ns-1/pod-1: vol-1
    pod-ns-1/pod-2: vol-2
`,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(tt *testing.T) {
			d := &Describer{
				Prefix: "",
				out:    &tabwriter.Writer{},
				buf:    &bytes.Buffer{},
			}
			d.out.Init(d.buf, 0, 8, 2, ' ', 0)
			DescribePodVolumeBackups(d, tc.inputPVBList, tc.inputDetails)
			d.out.Flush()
			assert.Equal(tt, tc.expect, d.buf.String())
		})
	}
}

func TestDescribeCSIVolumeSnapshots(t *testing.T) {
	features.Enable(velerov1api.CSIFeatureFlag)
	defer func() {
		features.Disable(velerov1api.CSIFeatureFlag)
	}()
	handle := "handle-1"
	readyToUse := true
	size := int64(1024)
	vsc1 := builder.ForVolumeSnapshotContent("vsc-1").
		Status(&snapshotv1api.VolumeSnapshotContentStatus{
			SnapshotHandle: &handle,
			ReadyToUse:     &readyToUse,
			RestoreSize:    &size,
		}).Result()
	testcases := []struct {
		name         string
		inputVSCList []snapshotv1api.VolumeSnapshotContent
		inputDetails bool
		expect       string
	}{
		{
			name:         "empty list",
			inputVSCList: []snapshotv1api.VolumeSnapshotContent{},
			inputDetails: false,
			expect: `CSI Volume Snapshots: <none included>
`,
		},
		{
			name:         "1 vsc no details",
			inputVSCList: []snapshotv1api.VolumeSnapshotContent{*vsc1},
			inputDetails: false,
			expect: `CSI Volume Snapshots:  1 included (specify --details for more information)
`,
		},
		{
			name:         "1 vsc with details",
			inputVSCList: []snapshotv1api.VolumeSnapshotContent{*vsc1},
			inputDetails: true,
			expect: `CSI Volume Snapshots:
Snapshot Content Name: vsc-1
  Storage Snapshot ID: handle-1
  Snapshot Size (bytes): 1024
  Ready to use: true
`,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(tt *testing.T) {
			d := &Describer{
				Prefix: "",
				out:    &tabwriter.Writer{},
				buf:    &bytes.Buffer{},
			}
			d.out.Init(d.buf, 0, 8, 2, ' ', 0)
			DescribeCSIVolumeSnapshots(d, tc.inputDetails, tc.inputVSCList)
			d.out.Flush()
			assert.Equal(tt, tc.expect, d.buf.String())
		})
	}
}
