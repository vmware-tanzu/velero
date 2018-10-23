/*
Copyright 2018 the Heptio Ark contributors.

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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	arkv1api "github.com/heptio/ark/pkg/apis/ark/v1"
	arktest "github.com/heptio/ark/pkg/util/test"
)

func TestPVBHandler(t *testing.T) {
	controllerNode := "foo"

	tests := []struct {
		name          string
		obj           *arkv1api.PodVolumeBackup
		shouldEnqueue bool
	}{
		{
			name: "Empty phase pvb on same node should be enqueued",
			obj: &arkv1api.PodVolumeBackup{
				Spec: arkv1api.PodVolumeBackupSpec{
					Node: controllerNode,
				},
			},
			shouldEnqueue: true,
		},
		{
			name: "New phase pvb on same node should be enqueued",
			obj: &arkv1api.PodVolumeBackup{
				Spec: arkv1api.PodVolumeBackupSpec{
					Node: controllerNode,
				},
				Status: arkv1api.PodVolumeBackupStatus{
					Phase: arkv1api.PodVolumeBackupPhaseNew,
				},
			},
			shouldEnqueue: true,
		},
		{
			name: "InProgress phase pvb on same node should not be enqueued",
			obj: &arkv1api.PodVolumeBackup{
				Spec: arkv1api.PodVolumeBackupSpec{
					Node: controllerNode,
				},
				Status: arkv1api.PodVolumeBackupStatus{
					Phase: arkv1api.PodVolumeBackupPhaseInProgress,
				},
			},
			shouldEnqueue: false,
		},
		{
			name: "Completed phase pvb on same node should not be enqueued",
			obj: &arkv1api.PodVolumeBackup{
				Spec: arkv1api.PodVolumeBackupSpec{
					Node: controllerNode,
				},
				Status: arkv1api.PodVolumeBackupStatus{
					Phase: arkv1api.PodVolumeBackupPhaseCompleted,
				},
			},
			shouldEnqueue: false,
		},
		{
			name: "Failed phase pvb on same node should not be enqueued",
			obj: &arkv1api.PodVolumeBackup{
				Spec: arkv1api.PodVolumeBackupSpec{
					Node: controllerNode,
				},
				Status: arkv1api.PodVolumeBackupStatus{
					Phase: arkv1api.PodVolumeBackupPhaseFailed,
				},
			},
			shouldEnqueue: false,
		},

		{
			name: "Empty phase pvb on different node should not be enqueued",
			obj: &arkv1api.PodVolumeBackup{
				Spec: arkv1api.PodVolumeBackupSpec{
					Node: "some-other-node",
				},
			},
			shouldEnqueue: false,
		},
		{
			name: "New phase pvb on different node should not be enqueued",
			obj: &arkv1api.PodVolumeBackup{
				Spec: arkv1api.PodVolumeBackupSpec{
					Node: "some-other-node",
				},
				Status: arkv1api.PodVolumeBackupStatus{
					Phase: arkv1api.PodVolumeBackupPhaseNew,
				},
			},
			shouldEnqueue: false,
		},
		{
			name: "InProgress phase pvb on different node should not be enqueued",
			obj: &arkv1api.PodVolumeBackup{
				Spec: arkv1api.PodVolumeBackupSpec{
					Node: "some-other-node",
				},
				Status: arkv1api.PodVolumeBackupStatus{
					Phase: arkv1api.PodVolumeBackupPhaseInProgress,
				},
			},
			shouldEnqueue: false,
		},
		{
			name: "Completed phase pvb on different node should not be enqueued",
			obj: &arkv1api.PodVolumeBackup{
				Spec: arkv1api.PodVolumeBackupSpec{
					Node: "some-other-node",
				},
				Status: arkv1api.PodVolumeBackupStatus{
					Phase: arkv1api.PodVolumeBackupPhaseCompleted,
				},
			},
			shouldEnqueue: false,
		},
		{
			name: "Failed phase pvb on different node should not be enqueued",
			obj: &arkv1api.PodVolumeBackup{
				Spec: arkv1api.PodVolumeBackupSpec{
					Node: "some-other-node",
				},
				Status: arkv1api.PodVolumeBackupStatus{
					Phase: arkv1api.PodVolumeBackupPhaseFailed,
				},
			},
			shouldEnqueue: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := &podVolumeBackupController{
				genericController: newGenericController("pod-volume-backup", arktest.NewLogger()),
				nodeName:          controllerNode,
			}

			c.pvbHandler(test.obj)

			if !test.shouldEnqueue {
				assert.Equal(t, 0, c.queue.Len())
				return
			}

			require.Equal(t, 1, c.queue.Len())
		})
	}
}
