/*
Copyright 2020 the Velero contributors.

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
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/fake"
)

const testNamespace = "velero"

func TestCreateOptions_BuildBackup(t *testing.T) {
	o := NewCreateOptions()
	o.Labels.Set("velero.io/test=true")
	o.OrderedResources = "pods=p1,p2;persistentvolumeclaims=pvc1,pvc2"
	orders, err := parseOrderedResources(o.OrderedResources)
	assert.NoError(t, err)

	backup, err := o.BuildBackup(testNamespace)
	assert.NoError(t, err)

	assert.Equal(t, velerov1api.BackupSpec{
		TTL:                     metav1.Duration{Duration: o.TTL},
		IncludedNamespaces:      []string(o.IncludeNamespaces),
		SnapshotVolumes:         o.SnapshotVolumes.Value,
		IncludeClusterResources: o.IncludeClusterResources.Value,
		OrderedResources:        orders,
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
		_, err := o.BuildBackup(testNamespace)
		assert.Error(t, err)
	})

	expectedBackupSpec := builder.ForBackup("test", testNamespace).IncludedNamespaces("test").Result().Spec
	schedule := builder.ForSchedule(testNamespace, "test").Template(expectedBackupSpec).ObjectMeta(builder.WithLabels("velero.io/test", "true"), builder.WithAnnotations("velero.io/test", "true")).Result()
	o.client.VeleroV1().Schedules(testNamespace).Create(context.TODO(), schedule, metav1.CreateOptions{})

	t.Run("existing schedule", func(t *testing.T) {
		backup, err := o.BuildBackup(testNamespace)
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
		backup, err := o.BuildBackup(testNamespace)
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
	orderedResources, err := parseOrderedResources("pods= ns1/p1; ns1/p2; persistentvolumeclaims=ns2/pvc1, ns2/pvc2")
	assert.NotNil(t, err)

	orderedResources, err = parseOrderedResources("pods= ns1/p1,ns1/p2 ; persistentvolumeclaims=ns2/pvc1,ns2/pvc2")
	assert.NoError(t, err)

	expectedResources := map[string]string{
		"pods":                   "ns1/p1,ns1/p2",
		"persistentvolumeclaims": "ns2/pvc1,ns2/pvc2",
	}
	assert.Equal(t, orderedResources, expectedResources)

	orderedResources, err = parseOrderedResources("pods= ns1/p1,ns1/p2 ; persistentvolumes=pv1,pv2")
	assert.NoError(t, err)

	expectedMixedResources := map[string]string{
		"pods":              "ns1/p1,ns1/p2",
		"persistentvolumes": "pv1,pv2",
	}
	assert.Equal(t, orderedResources, expectedMixedResources)

}
