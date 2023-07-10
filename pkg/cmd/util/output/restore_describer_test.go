package output

import (
	"bytes"
	"testing"
	"text/tabwriter"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/itemoperation"
	"github.com/vmware-tanzu/velero/pkg/util/results"
)

func TestDescribeResult(t *testing.T) {
	testcases := []struct {
		name        string
		inputName   string
		inputResult results.Result
		expect      string
	}{
		{
			name:      "result without ns warns",
			inputName: "restore-1",
			inputResult: results.Result{
				Velero:     []string{"velero-msg-1", "velero-msg-2"},
				Cluster:    []string{"cluster-msg-1", "cluster-msg-2"},
				Namespaces: map[string][]string{},
			},
			expect: `restore-1:
  Velero:   velero-msg-1
            velero-msg-2
  Cluster:  cluster-msg-1
            cluster-msg-2
  Namespaces: <none>
`,
		},
		{
			name:      "result with ns warns",
			inputName: "restore-2",
			inputResult: results.Result{
				Velero:  []string{"velero-msg-1", "velero-msg-2"},
				Cluster: []string{"cluster-msg-1", "cluster-msg-2"},
				Namespaces: map[string][]string{
					"ns-1": {"ns-1-warn-1", "ns-1-warn-2"},
				},
			},
			expect: `restore-2:
  Velero:   velero-msg-1
            velero-msg-2
  Cluster:  cluster-msg-1
            cluster-msg-2
  Namespaces:
    ns-1:  ns-1-warn-1
           ns-1-warn-2
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
			describeResult(d, tc.inputName, tc.inputResult)
			d.out.Flush()
			assert.Equal(tt, tc.expect, d.buf.String())
		})
	}
}

func TestDescribeRestoreItemOperation(t *testing.T) {
	t1, err1 := time.Parse("2006-Jan-02", "2023-Jun-26")
	require.Nil(t, err1)
	t2, err2 := time.Parse("2006-Jan-02", "2023-Jun-25")
	require.Nil(t, err2)
	t3, err3 := time.Parse("2006-Jan-02", "2023-Jun-24")
	require.Nil(t, err3)
	input := builder.ForRestoreOperation().
		RestoreName("restore-1").
		OperationID("op-1").
		RestoreItemAction("action-1").
		ResourceIdentifier("group", "rs-type", "ns", "rs-name").
		Status(*builder.ForOperationStatus().
			Phase(itemoperation.OperationPhaseFailed).
			Error("operation error").
			Progress(50, 100, "bytes").
			Description("operation description").
			Created(t3).
			Started(t2).
			Updated(t1).
			Result()).Result()
	expected := `  Operation for rs-type.group ns/rs-name:
    Restore Item Action Plugin:  action-1
    Operation ID:                op-1
    Phase:                       Failed
    Operation Error:             operation error
    Progress:                    50 of 100 complete (bytes)
    Progress description:        operation description
    Created:                     2023-06-24 00:00:00 +0000 UTC
    Started:                     2023-06-25 00:00:00 +0000 UTC
    Updated:                     2023-06-26 00:00:00 +0000 UTC
`
	d := &Describer{
		Prefix: "",
		out:    &tabwriter.Writer{},
		buf:    &bytes.Buffer{},
	}
	d.out.Init(d.buf, 0, 8, 2, ' ', 0)
	describeRestoreItemOperation(d, input)
	d.out.Flush()
	assert.Equal(t, expected, d.buf.String())
}

func TestDescribePodVolumeRestores(t *testing.T) {
	pvr1 := builder.ForPodVolumeRestore("velero", "pvr-1").
		UploaderType("kopia").
		Phase(velerov1api.PodVolumeRestorePhaseCompleted).
		BackupStorageLocation("bsl-1").
		Volume("vol-1").
		PodName("pod-1").
		PodNamespace("pod-ns-1").
		SnapshotID("snap-1").Result()
	pvr2 := builder.ForPodVolumeRestore("velero", "pvr-2").
		UploaderType("kopia").
		Phase(velerov1api.PodVolumeRestorePhaseCompleted).
		BackupStorageLocation("bsl-1").
		Volume("vol-2").
		PodName("pod-2").
		PodNamespace("pod-ns-1").
		SnapshotID("snap-2").Result()

	testcases := []struct {
		name         string
		inputPVRList []velerov1api.PodVolumeRestore
		inputDetails bool
		expect       string
	}{
		{
			name:         "empty list",
			inputPVRList: []velerov1api.PodVolumeRestore{},
			inputDetails: true,
			expect:       ``,
		},
		{
			name:         "2 completed pvrs no details",
			inputPVRList: []velerov1api.PodVolumeRestore{*pvr1, *pvr2},
			inputDetails: false,
			expect: `kopia Restores (specify --details for more information):
  Completed:  2
`,
		},
		{
			name:         "2 completed pvrs with details",
			inputPVRList: []velerov1api.PodVolumeRestore{*pvr1, *pvr2},
			inputDetails: true,
			expect: `kopia Restores:
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
			describePodVolumeRestores(d, tc.inputPVRList, tc.inputDetails)
			d.out.Flush()
			assert.Equal(tt, tc.expect, d.buf.String())
		})
	}
}
