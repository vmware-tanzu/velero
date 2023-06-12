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
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	flag "github.com/spf13/pflag"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/client"
	clicmd "github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/fake"
)

func TestCreateOptions_BuildBackup(t *testing.T) {
	o := NewCreateOptions()
	o.Labels.Set("velero.io/test=true")
	o.OrderedResources = "pods=p1,p2;persistentvolumeclaims=pvc1,pvc2"
	orders, err := ParseOrderedResources(o.OrderedResources)
	o.CSISnapshotTimeout = 20 * time.Minute
	o.ItemOperationTimeout = 20 * time.Minute
	assert.NoError(t, err)

	backup, err := o.BuildBackup(clicmd.VeleroNameSpace)
	assert.NoError(t, err)

	assert.Equal(t, velerov1api.BackupSpec{
		TTL:                     metav1.Duration{Duration: o.TTL},
		IncludedNamespaces:      []string(o.IncludeNamespaces),
		SnapshotVolumes:         o.SnapshotVolumes.Value,
		IncludeClusterResources: o.IncludeClusterResources.Value,
		OrderedResources:        orders,
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
	o.client = fake.NewSimpleClientset()

	t.Run("inexistent schedule", func(t *testing.T) {
		_, err := o.BuildBackup(clicmd.VeleroNameSpace)
		assert.Error(t, err)
	})

	expectedBackupSpec := builder.ForBackup("test", clicmd.VeleroNameSpace).IncludedNamespaces("test").Result().Spec
	schedule := builder.ForSchedule(clicmd.VeleroNameSpace, "test").Template(expectedBackupSpec).ObjectMeta(builder.WithLabels("velero.io/test", "true"), builder.WithAnnotations("velero.io/test", "true")).Result()
	o.client.VeleroV1().Schedules(clicmd.VeleroNameSpace).Create(context.TODO(), schedule, metav1.CreateOptions{})

	t.Run("existing schedule", func(t *testing.T) {
		backup, err := o.BuildBackup(clicmd.VeleroNameSpace)
		assert.NoError(t, err)

		assert.Equal(t, expectedBackupSpec, backup.Spec)
		assert.Equal(t, map[string]string{
			"velero.io/test":              "true",
			velerov1api.ScheduleNameLabel: "test",
		}, backup.GetLabels())
		assert.Equal(t, map[string]string{
			"velero.io/test": "true",
		}, backup.GetAnnotations())
	})

	t.Run("command line labels take precedence over schedule labels", func(t *testing.T) {
		o.Labels.Set("velero.io/test=yes,custom-label=true")
		backup, err := o.BuildBackup(clicmd.VeleroNameSpace)
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
	assert.NotNil(t, err)

	orderedResources, err := ParseOrderedResources("pods= ns1/p1,ns1/p2 ; persistentvolumeclaims=ns2/pvc1,ns2/pvc2")
	assert.NoError(t, err)

	expectedResources := map[string]string{
		"pods":                   "ns1/p1,ns1/p2",
		"persistentvolumeclaims": "ns2/pvc1,ns2/pvc2",
	}
	assert.Equal(t, orderedResources, expectedResources)

	orderedResources, err = ParseOrderedResources("pods= ns1/p1,ns1/p2 ; persistentvolumes=pv1,pv2")
	assert.NoError(t, err)

	expectedMixedResources := map[string]string{
		"pods":              "ns1/p1,ns1/p2",
		"persistentvolumes": "pv1,pv2",
	}
	assert.Equal(t, orderedResources, expectedMixedResources)

}

func TestCreateCommand_Run(t *testing.T) {

	// create a config for factory
	baseName := "velero-bn"
	os.Setenv("VELERO_NAMESPACE", clicmd.VeleroNameSpace)
	config, err := client.LoadConfig()
	assert.Equal(t, err, nil)

	// create a factory
	f := client.NewFactory(baseName, config)
	cliFlags := new(flag.FlagSet)
	f.BindFlags(cliFlags)
	cliFlags.Parse(clicmd.FactoryFlags)

	// create command
	cmd := NewCreateCommand(f, "")
	assert.Equal(t, "Create a backup", cmd.Short)

	// create a CreateOptions with full options set and then run this backup command
	name := "nameToBeCreated"
	includeNamespaces := "app1,app2"
	excludeNamespaces := "pod1,pod2,pod3"
	includeResources := "sc,sts"
	excludeResources := "job"
	includeClusterScopedResources := "pv,ComponentStatus"
	excludeClusterScopedResources := "MutatingWebhookConfiguration,APIService"
	includeNamespaceScopedResources := "Endpoints,Event,PodTemplate"
	excludeNamespaceScopedResources := "Secret,MultiClusterIngress"
	labels := "c=foo,b=woo"
	storageLocation := "bsl-name-1"
	snapshotLocations := "region=minio"
	selector := "a=pod"
	orderedResources := "bsl-name-1"
	csiSnapshotTimeout := "8m30s"
	itemOperationTimeout := "99h1m6s"
	snapshotVolumes := "false"
	snapshotMoveData := "true"
	includeClusterResources := "true"
	defaultVolumesToFsBackup := "true"
	resPoliciesConfigmap := "cm-name-2"
	dataMover := "velero"
	fromSchedule := "schedule-name-1"

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
	flags.Parse([]string{"--from-schedule", fromSchedule})
	flags.Parse([]string{"--wait"})

	args := []string{name, "arg2"}
	o.Complete(args, f)
	e := o.Validate(cmd, args, f)
	assert.Contains(t, e.Error(), "/api?timeout=")

	e = o.Run(cmd, f)
	// as fromSchedule is set, so the error below is expected
	assert.Contains(t, e.Error(), fmt.Sprintf("Get \"%s/apis/velero.io/v1/namespaces/%s/schedules/%s", clicmd.HOST, clicmd.VeleroNameSpace, fromSchedule))

	// verify all options are set as expected
	assert.Equal(t, name, o.Name)
	assert.Equal(t, includeNamespaces, o.IncludeNamespaces.String())
	assert.Equal(t, excludeNamespaces, o.ExcludeNamespaces.String())
	assert.Equal(t, includeResources, o.IncludeResources.String())
	assert.Equal(t, excludeResources, o.ExcludeResources.String())
	assert.Equal(t, includeClusterScopedResources, o.IncludeClusterScopedResources.String())
	assert.Equal(t, excludeClusterScopedResources, o.ExcludeClusterScopedResources.String())
	assert.Equal(t, includeNamespaceScopedResources, o.IncludeNamespaceScopedResources.String())
	assert.Equal(t, excludeNamespaceScopedResources, o.ExcludeNamespaceScopedResources.String())
	assert.Equal(t, true, clicmd.CompareSlice(strings.Split(labels, ","), strings.Split(o.Labels.String(), ",")))
	assert.Equal(t, storageLocation, o.StorageLocation)
	assert.Equal(t, snapshotLocations, strings.Split(o.SnapshotLocations[0], ",")[0])
	assert.Equal(t, selector, o.Selector.String())
	assert.Equal(t, orderedResources, o.OrderedResources)
	assert.Equal(t, csiSnapshotTimeout, o.CSISnapshotTimeout.String())
	assert.Equal(t, itemOperationTimeout, o.ItemOperationTimeout.String())
	assert.Equal(t, snapshotVolumes, o.SnapshotVolumes.String())
	assert.Equal(t, snapshotMoveData, o.SnapshotMoveData.String())
	assert.Equal(t, includeClusterResources, o.IncludeClusterResources.String())
	assert.Equal(t, defaultVolumesToFsBackup, o.DefaultVolumesToFsBackup.String())
	assert.Equal(t, resPoliciesConfigmap, o.ResPoliciesConfigmap)
	assert.Equal(t, dataMover, o.DataMover)
	assert.Equal(t, fromSchedule, o.FromSchedule)
	assert.Equal(t, true, o.Wait)

	// verify oldAndNewFilterParametersUsedTogether
	mix := o.oldAndNewFilterParametersUsedTogether()
	assert.Equal(t, true, mix)

	// create the other create command without fromSchedule option for Run() other branches
	cmd = NewCreateCommand(f, "velero backup create")
	assert.Equal(t, "Create a backup", cmd.Short)

	o = NewCreateOptions()
	o.Labels.Set("velero.io/test=true")
	o.OrderedResources = "pods=p1,p2;persistentvolumeclaims=pvc1,pvc2"
	o.CSISnapshotTimeout = 20 * time.Minute
	o.ItemOperationTimeout = 20 * time.Minute
	o.Wait = true
	f.BindFlags(cliFlags)
	args = []string{"backup-name-2", "arg2"}
	e = o.Complete(args, f)
	assert.NoError(t, e)
	fmt.Println(o.client)
	e = o.Run(cmd, f)

	// Get failure of backup resource creation
	assert.Contains(t, e.Error(), fmt.Sprintf("Post \"%s/apis/velero.io/v1/namespaces/%s/backups", clicmd.HOST, clicmd.VeleroNameSpace))
}

func TestCreateCommand_Execute(t *testing.T) {
	if os.Getenv(clicmd.TestExitFlag) == "1" {
		// create a config for factory
		config, err := client.LoadConfig()
		assert.Equal(t, err, nil)

		// create a factory
		f := client.NewFactory("velero-bn", config)

		// create command
		cmd := NewCreateCommand(f, "")
		cmd.Execute()
		return
	}
	clicmd.TestProcessExit(t, exec.Command(os.Args[0], []string{"-test.run=TestCreateCommand_Execute"}...))
}
