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

package backup

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	flag "github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	controllerclient "sigs.k8s.io/controller-runtime/pkg/client"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	factorymocks "github.com/vmware-tanzu/velero/pkg/client/mocks"
	cmdtest "github.com/vmware-tanzu/velero/pkg/cmd/test"
	"github.com/vmware-tanzu/velero/pkg/test"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestCreateOptions_BuildBackup(t *testing.T) {
	o := NewCreateOptions()
	o.Labels.Set("velero.io/test=true")
	o.OrderedResources = "pods=p1,p2;persistentvolumeclaims=pvc1,pvc2"
	orders, err := ParseOrderedResources(o.OrderedResources)
	o.CSISnapshotTimeout = 20 * time.Minute
	o.ItemOperationTimeout = 20 * time.Minute
	orLabelSelectors := []*metav1.LabelSelector{
		{
			MatchLabels: map[string]string{"k1": "v1", "k2": "v2"},
		},
		{
			MatchLabels: map[string]string{"a1": "b1", "a2": "b2"},
		},
	}
	o.OrSelector.OrLabelSelectors = orLabelSelectors
	assert.NoError(t, err)

	backup, err := o.BuildBackup(cmdtest.VeleroNameSpace)
	assert.NoError(t, err)

	assert.Equal(t, velerov1api.BackupSpec{
		TTL:                     metav1.Duration{Duration: o.TTL},
		IncludedNamespaces:      []string(o.IncludeNamespaces),
		SnapshotVolumes:         o.SnapshotVolumes.Value,
		IncludeClusterResources: o.IncludeClusterResources.Value,
		OrderedResources:        orders,
		OrLabelSelectors:        orLabelSelectors,
		CSISnapshotTimeout:      metav1.Duration{Duration: o.CSISnapshotTimeout},
		ItemOperationTimeout:    metav1.Duration{Duration: o.ItemOperationTimeout},
	}, backup.Spec)

	assert.Equal(t, map[string]string{
		"velero.io/test": "true",
	}, backup.GetLabels())
	assert.Equal(t, map[string]string{
		"pods":                   "p1,p2",
		"persistentvolumeclaims": "pvc1,pvc2",
	}, backup.Spec.OrderedResources)
}

func TestCreateOptions_BuildBackupFromSchedule(t *testing.T) {
	o := NewCreateOptions()
	o.FromSchedule = "test"

	scheme := runtime.NewScheme()
	err := velerov1api.AddToScheme(scheme)
	require.NoError(t, err)
	o.client = velerotest.NewFakeControllerRuntimeClient(t).(controllerclient.WithWatch)

	t.Run("inexistent schedule", func(t *testing.T) {
		_, err := o.BuildBackup(cmdtest.VeleroNameSpace)
		require.Error(t, err)
	})

	expectedBackupSpec := builder.ForBackup("test", cmdtest.VeleroNameSpace).IncludedNamespaces("test").Result().Spec
	schedule := builder.ForSchedule(cmdtest.VeleroNameSpace, "test").Template(expectedBackupSpec).ObjectMeta(builder.WithLabels("velero.io/test", "true"), builder.WithAnnotations("velero.io/test", "true")).Result()
	o.client.Create(context.TODO(), schedule, &kbclient.CreateOptions{})

	t.Run("existing schedule", func(t *testing.T) {
		backup, err := o.BuildBackup(cmdtest.VeleroNameSpace)
		require.NoError(t, err)

		require.Equal(t, expectedBackupSpec, backup.Spec)
		require.Equal(t, map[string]string{
			"velero.io/test":              "true",
			velerov1api.ScheduleNameLabel: "test",
		}, backup.GetLabels())
		require.Equal(t, map[string]string{
			"velero.io/test": "true",
		}, backup.GetAnnotations())
	})

	t.Run("command line labels take precedence over schedule labels", func(t *testing.T) {
		o.Labels.Set("velero.io/test=yes,custom-label=true")
		backup, err := o.BuildBackup(cmdtest.VeleroNameSpace)
		assert.NoError(t, err)

		assert.Equal(t, expectedBackupSpec, backup.Spec)
		assert.Equal(t, map[string]string{
			"velero.io/test":              "yes",
			velerov1api.ScheduleNameLabel: "test",
			"custom-label":                "true",
		}, backup.GetLabels())
	})
}

func TestCreateOptions_OrderedResources(t *testing.T) {
	_, err := ParseOrderedResources("pods= ns1/p1; ns1/p2; persistentvolumeclaims=ns2/pvc1, ns2/pvc2")
	assert.Error(t, err)

	orderedResources, err := ParseOrderedResources("pods= ns1/p1,ns1/p2 ; persistentvolumeclaims=ns2/pvc1,ns2/pvc2")
	assert.NoError(t, err)

	expectedResources := map[string]string{
		"pods":                   "ns1/p1,ns1/p2",
		"persistentvolumeclaims": "ns2/pvc1,ns2/pvc2",
	}
	assert.Equal(t, expectedResources, orderedResources)

	orderedResources, err = ParseOrderedResources("pods= ns1/p1,ns1/p2 ; persistentvolumes=pv1,pv2")
	assert.NoError(t, err)

	expectedMixedResources := map[string]string{
		"pods":              "ns1/p1,ns1/p2",
		"persistentvolumes": "pv1,pv2",
	}
	assert.Equal(t, expectedMixedResources, orderedResources)
}

func TestCreateCommand(t *testing.T) {
	name := "nameToBeCreated"
	args := []string{name}

	t.Run("create a backup create command with full options except fromSchedule and wait, then run by create option", func(t *testing.T) {
		// create a factory
		f := &factorymocks.Factory{}

		// create command
		cmd := NewCreateCommand(f, "")
		assert.Equal(t, "Create a backup", cmd.Short)

		includeNamespaces := "app1,app2"
		excludeNamespaces := "pod1,pod2,pod3"
		includeResources := "sc,sts"
		excludeResources := "job"
		includeClusterScopedResources := "pv,ComponentStatus"
		excludeClusterScopedResources := "MutatingWebhookConfiguration,APIService"
		includeNamespaceScopedResources := "Endpoints,Event,PodTemplate"
		excludeNamespaceScopedResources := "Secret,MultiClusterIngress"
		labels := "c=foo"
		storageLocation := "bsl-name-1"
		snapshotLocations := "region=minio"
		selector := "a=pod"
		orderedResources := "pod=pod1,pod2,pod3"
		csiSnapshotTimeout := "8m30s"
		itemOperationTimeout := "99h1m6s"
		snapshotVolumes := "false"
		snapshotMoveData := "true"
		includeClusterResources := "true"
		defaultVolumesToFsBackup := "true"
		resPoliciesConfigmap := "cm-name-2"
		dataMover := "velero"
		parallelFilesUpload := 10
		flags := new(flag.FlagSet)
		o := NewCreateOptions()
		o.BindFlags(flags)
		o.BindWait(flags)
		o.BindFromSchedule(flags)

		flags.Parse([]string{"--include-namespaces", includeNamespaces})
		flags.Parse([]string{"--exclude-namespaces", excludeNamespaces})
		flags.Parse([]string{"--include-resources", includeResources})
		flags.Parse([]string{"--exclude-resources", excludeResources})
		flags.Parse([]string{"--include-cluster-scoped-resources", includeClusterScopedResources})
		flags.Parse([]string{"--exclude-cluster-scoped-resources", excludeClusterScopedResources})
		flags.Parse([]string{"--include-namespace-scoped-resources", includeNamespaceScopedResources})
		flags.Parse([]string{"--exclude-namespace-scoped-resources", excludeNamespaceScopedResources})
		flags.Parse([]string{"--labels", labels})
		flags.Parse([]string{"--storage-location", storageLocation})
		flags.Parse([]string{"--volume-snapshot-locations", snapshotLocations})
		flags.Parse([]string{"--selector", selector})
		flags.Parse([]string{"--ordered-resources", orderedResources})
		flags.Parse([]string{"--csi-snapshot-timeout", csiSnapshotTimeout})
		flags.Parse([]string{"--item-operation-timeout", itemOperationTimeout})
		flags.Parse([]string{fmt.Sprintf("--snapshot-volumes=%s", snapshotVolumes)})
		flags.Parse([]string{fmt.Sprintf("--snapshot-move-data=%s", snapshotMoveData)})
		flags.Parse([]string{"--include-cluster-resources", includeClusterResources})
		flags.Parse([]string{"--default-volumes-to-fs-backup", defaultVolumesToFsBackup})
		flags.Parse([]string{"--resource-policies-configmap", resPoliciesConfigmap})
		flags.Parse([]string{"--data-mover", dataMover})
		flags.Parse([]string{"--parallel-files-upload", fmt.Sprintf("%d", parallelFilesUpload)})
		//flags.Parse([]string{"--wait"})

		client := velerotest.NewFakeControllerRuntimeClient(t).(kbclient.WithWatch)

		f.On("Namespace").Return(mock.Anything)
		f.On("KubebuilderWatchClient").Return(client, nil)

		//Complete
		e := o.Complete(args, f)
		require.NoError(t, e)

		//Validate
		e = o.Validate(cmd, args, f)
		require.Contains(t, e.Error(), "include-resources, exclude-resources and include-cluster-resources are old filter parameters")
		require.Contains(t, e.Error(), "include-cluster-scoped-resources, exclude-cluster-scoped-resources, include-namespace-scoped-resources and exclude-namespace-scoped-resources are new filter parameters.\nThey cannot be used together")

		//cmd
		e = o.Run(cmd, f)
		require.NoError(t, e)

		//Execute
		cmd.SetArgs([]string{"bk-name-exe"})
		e = cmd.Execute()
		require.NoError(t, e)

		// verify all options are set as expected
		require.Equal(t, name, o.Name)
		require.Equal(t, includeNamespaces, o.IncludeNamespaces.String())
		require.Equal(t, excludeNamespaces, o.ExcludeNamespaces.String())
		require.Equal(t, includeResources, o.IncludeResources.String())
		require.Equal(t, excludeResources, o.ExcludeResources.String())
		require.Equal(t, includeClusterScopedResources, o.IncludeClusterScopedResources.String())
		require.Equal(t, excludeClusterScopedResources, o.ExcludeClusterScopedResources.String())
		require.Equal(t, includeNamespaceScopedResources, o.IncludeNamespaceScopedResources.String())
		require.Equal(t, excludeNamespaceScopedResources, o.ExcludeNamespaceScopedResources.String())
		require.True(t, test.CompareSlice(strings.Split(labels, ","), strings.Split(o.Labels.String(), ",")))
		require.Equal(t, storageLocation, o.StorageLocation)
		require.Equal(t, snapshotLocations, strings.Split(o.SnapshotLocations[0], ",")[0])
		require.Equal(t, selector, o.Selector.String())
		require.Equal(t, orderedResources, o.OrderedResources)
		require.Equal(t, csiSnapshotTimeout, o.CSISnapshotTimeout.String())
		require.Equal(t, itemOperationTimeout, o.ItemOperationTimeout.String())
		require.Equal(t, snapshotVolumes, o.SnapshotVolumes.String())
		require.Equal(t, snapshotMoveData, o.SnapshotMoveData.String())
		require.Equal(t, includeClusterResources, o.IncludeClusterResources.String())
		require.Equal(t, defaultVolumesToFsBackup, o.DefaultVolumesToFsBackup.String())
		require.Equal(t, resPoliciesConfigmap, o.ResPoliciesConfigmap)
		require.Equal(t, dataMover, o.DataMover)
		require.Equal(t, parallelFilesUpload, o.ParallelFilesUpload)
		//assert.Equal(t, true, o.Wait)

		// verify oldAndNewFilterParametersUsedTogether
		mix := o.oldAndNewFilterParametersUsedTogether()
		require.True(t, mix)
	})

	t.Run("create a backup create command with specific storage-location setting", func(t *testing.T) {
		bsl := "bsl-1"
		// create a factory
		f := &factorymocks.Factory{}
		cmd := NewCreateCommand(f, "")
		kbclient := velerotest.NewFakeControllerRuntimeClient(t).(kbclient.WithWatch)

		f.On("Namespace").Return(mock.Anything)
		f.On("KubebuilderWatchClient").Return(kbclient, nil)

		flags := new(flag.FlagSet)
		o := NewCreateOptions()
		o.BindFlags(flags)
		o.BindWait(flags)
		o.BindFromSchedule(flags)
		flags.Parse([]string{"--include-namespaces", "ns-1"})
		flags.Parse([]string{"--storage-location", bsl})

		// Complete
		e := o.Complete(args, f)
		assert.NoError(t, e)

		// Validate
		e = o.Validate(cmd, args, f)
		assert.Contains(t, e.Error(), fmt.Sprintf("backupstoragelocations.velero.io \"%s\" not found", bsl))
	})

	t.Run("create a backup create command with specific volume-snapshot-locations setting", func(t *testing.T) {
		vslName := "vsl-1"
		// create a factory
		f := &factorymocks.Factory{}
		cmd := NewCreateCommand(f, "")
		kbclient := velerotest.NewFakeControllerRuntimeClient(t).(kbclient.WithWatch)

		vsl := builder.ForVolumeSnapshotLocation(cmdtest.VeleroNameSpace, vslName).Result()

		kbclient.Create(cmd.Context(), vsl, &controllerclient.CreateOptions{})

		f.On("Namespace").Return(cmdtest.VeleroNameSpace)
		f.On("KubebuilderWatchClient").Return(kbclient, nil)

		flags := new(flag.FlagSet)
		o := NewCreateOptions()
		o.BindFlags(flags)
		o.BindWait(flags)
		o.BindFromSchedule(flags)
		flags.Parse([]string{"--include-namespaces", "ns-1"})
		flags.Parse([]string{"--volume-snapshot-locations", vslName})

		// Complete
		e := o.Complete(args, f)
		assert.NoError(t, e)

		// Validate
		e = o.Validate(cmd, args, f)
		assert.NoError(t, e)
	})

	t.Run("create the other create command with fromSchedule option for Run() other branches", func(t *testing.T) {
		f := &factorymocks.Factory{}
		c := NewCreateCommand(f, "")
		assert.Equal(t, "Create a backup", c.Short)
		flags := new(flag.FlagSet)
		o := NewCreateOptions()
		o.BindFlags(flags)
		o.BindWait(flags)
		o.BindFromSchedule(flags)
		fromSchedule := "schedule-name-1"
		flags.Parse([]string{"--from-schedule", fromSchedule})

		kbclient := velerotest.NewFakeControllerRuntimeClient(t).(kbclient.WithWatch)

		schedule := builder.ForSchedule(cmdtest.VeleroNameSpace, fromSchedule).Result()
		kbclient.Create(context.Background(), schedule, &controllerclient.CreateOptions{})

		f.On("Namespace").Return(cmdtest.VeleroNameSpace)
		f.On("KubebuilderWatchClient").Return(kbclient, nil)

		e := o.Complete(args, f)
		assert.NoError(t, e)

		e = o.Run(c, f)
		assert.NoError(t, e)

		c.SetArgs([]string{"bk-1"})
		e = c.Execute()
		assert.NoError(t, e)
	})
}
