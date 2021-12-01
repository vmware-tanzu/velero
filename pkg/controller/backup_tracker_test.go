/*
Copyright 2018 the Velero contributors.

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

package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/backup"

	"github.com/stretchr/testify/assert"
)

func TestBackupTracker(t *testing.T) {
	bt := NewBackupTracker()

	assert.False(t, bt.Contains("ns", "name"))

	name1Backup := &backup.Request{
		Backup: &velerov1api.Backup{
			ObjectMeta: metav1.ObjectMeta{Name: "name"},
		},
	}
	bt.Add("ns", "name", name1Backup)
	assert.True(t, bt.Contains("ns", "name"))
	assert.Equal(t, name1Backup, bt.Get("ns", "name"))

	name2Backup := &backup.Request{
		Backup: &velerov1api.Backup{
			ObjectMeta: metav1.ObjectMeta{Name: "name2"},
		},
	}

	bt.Add("ns2", "name2", name2Backup)
	assert.True(t, bt.Contains("ns", "name"))
	assert.True(t, bt.Contains("ns2", "name2"))

	assert.Equal(t, name1Backup, bt.Get("ns", "name"))
	assert.Equal(t, name2Backup, bt.Get("ns2", "name2"))

	bt.Delete("ns", "name")
	assert.False(t, bt.Contains("ns", "name"))
	assert.True(t, bt.Contains("ns2", "name2"))

	bt.Delete("ns2", "name2")
	assert.False(t, bt.Contains("ns2", "name2"))

	assert.Equal(t, 0, len(bt.GetByPhase(velerov1api.BackupPhaseUploading)))
	name3PhaseUploading := &backup.Request{
		Backup: &velerov1api.Backup{
			ObjectMeta: metav1.ObjectMeta{Name: "name3"},
			Status: velerov1api.BackupStatus{
				Phase: velerov1api.BackupPhaseUploading,
			},
		},
	}
	bt.Add("ns", "name3", name3PhaseUploading)
	reqs := bt.GetByPhase(velerov1api.BackupPhaseUploading)
	assert.Equal(t, 1, len(reqs))
	assert.Equal(t, reqs[0].Name, "name3")
	assert.Equal(t, velerov1api.BackupPhaseUploading, reqs[0].Status.Phase)

}
