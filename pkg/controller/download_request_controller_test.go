/*
Copyright 2017 Heptio Inc.

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
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	core "k8s.io/client-go/testing"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/generated/clientset/versioned/fake"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions"
	"github.com/heptio/ark/pkg/util/collections"
	arktest "github.com/heptio/ark/pkg/util/test"
)

func TestProcessDownloadRequest(t *testing.T) {
	tests := []struct {
		name          string
		key           string
		phase         v1.DownloadRequestPhase
		targetKind    v1.DownloadTargetKind
		targetName    string
		expectedError string
		expectedPhase v1.DownloadRequestPhase
		expectedURL   string
	}{
		{
			name: "empty key",
			key:  "",
		},
		{
			name:          "bad key format",
			key:           "a/b/c",
			expectedError: `error splitting queue key: unexpected key format: "a/b/c"`,
		},
		{
			name:          "backup log request with phase '' gets a url",
			key:           "heptio-ark/dr1",
			phase:         "",
			targetKind:    v1.DownloadTargetKindBackupLog,
			targetName:    "backup1",
			expectedPhase: v1.DownloadRequestPhaseProcessed,
			expectedURL:   "signedURL",
		},
		{
			name:          "backup log request with phase 'New' gets a url",
			key:           "heptio-ark/dr1",
			phase:         v1.DownloadRequestPhaseNew,
			targetKind:    v1.DownloadTargetKindBackupLog,
			targetName:    "backup1",
			expectedPhase: v1.DownloadRequestPhaseProcessed,
			expectedURL:   "signedURL",
		},
		{
			name:          "restore log request with phase '' gets a url",
			key:           "heptio-ark/dr1",
			phase:         "",
			targetKind:    v1.DownloadTargetKindRestoreLog,
			targetName:    "backup1-20170912150214",
			expectedPhase: v1.DownloadRequestPhaseProcessed,
			expectedURL:   "signedURL",
		},
		{
			name:          "restore log request with phase New gets a url",
			key:           "heptio-ark/dr1",
			phase:         v1.DownloadRequestPhaseNew,
			targetKind:    v1.DownloadTargetKindRestoreLog,
			targetName:    "backup1-20170912150214",
			expectedPhase: v1.DownloadRequestPhaseProcessed,
			expectedURL:   "signedURL",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				client                   = fake.NewSimpleClientset()
				sharedInformers          = informers.NewSharedInformerFactory(client, 0)
				downloadRequestsInformer = sharedInformers.Ark().V1().DownloadRequests()
				backupService            = &arktest.BackupService{}
				logger                   = arktest.NewLogger()
			)
			defer backupService.AssertExpectations(t)

			c := NewDownloadRequestController(
				client.ArkV1(),
				downloadRequestsInformer,
				backupService,
				"bucket",
				logger,
			).(*downloadRequestController)

			var downloadRequest *v1.DownloadRequest

			if tc.expectedPhase == v1.DownloadRequestPhaseProcessed {
				target := v1.DownloadTarget{
					Kind: tc.targetKind,
					Name: tc.targetName,
				}

				downloadRequest = &v1.DownloadRequest{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: v1.DefaultNamespace,
						Name:      "dr1",
					},
					Spec: v1.DownloadRequestSpec{
						Target: target,
					},
				}
				downloadRequestsInformer.Informer().GetStore().Add(downloadRequest)

				backupService.On("CreateSignedURL", target, "bucket", 10*time.Minute).Return("signedURL", nil)
			}

			// method under test
			err := c.processDownloadRequest(tc.key)

			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)

			actions := client.Actions()

			// if we don't expect a phase update, this means
			// we don't expect any actions to take place
			if tc.expectedPhase == "" {
				require.Equal(t, 0, len(actions))
				return
			}

			// otherwise, we should get exactly 1 patch
			require.Equal(t, 1, len(actions))
			patchAction, ok := actions[0].(core.PatchAction)
			require.True(t, ok, "action is not a PatchAction")

			patch := make(map[string]interface{})
			require.NoError(t, json.Unmarshal(patchAction.GetPatch(), &patch), "cannot unmarshal patch")

			// check the URL
			assert.True(t, collections.HasKeyAndVal(patch, "status.downloadURL", tc.expectedURL), "patch's status.downloadURL does not match")

			// check the Phase
			assert.True(t, collections.HasKeyAndVal(patch, "status.phase", string(tc.expectedPhase)), "patch's status.phase does not match")

			// check that Expiration exists
			// TODO pass a fake clock to the controller and verify
			// the expiration value
			assert.True(t, collections.Exists(patch, "status.expiration"), "patch's status.expiration does not exist")

			// we expect 3 total updates.
			res, _ := collections.GetMap(patch, "status")
			assert.Equal(t, 3, len(res), "patch's status has the wrong number of keys")
		})
	}
}
