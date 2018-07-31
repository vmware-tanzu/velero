/*
Copyright 2017 the Heptio Ark contributors.

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
	"k8s.io/apimachinery/pkg/util/clock"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/generated/clientset/versioned/fake"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions"
	arktest "github.com/heptio/ark/pkg/util/test"
)

func TestProcessDownloadRequest(t *testing.T) {
	tests := []struct {
		name          string
		key           string
		phase         v1.DownloadRequestPhase
		targetKind    v1.DownloadTargetKind
		targetName    string
		restore       *v1.Restore
		expectedError string
		expectedDir   string
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
			expectedDir:   "backup1",
			expectedPhase: v1.DownloadRequestPhaseProcessed,
			expectedURL:   "signedURL",
		},
		{
			name:          "backup log request with phase 'New' gets a url",
			key:           "heptio-ark/dr1",
			phase:         v1.DownloadRequestPhaseNew,
			targetKind:    v1.DownloadTargetKindBackupLog,
			targetName:    "backup1",
			expectedDir:   "backup1",
			expectedPhase: v1.DownloadRequestPhaseProcessed,
			expectedURL:   "signedURL",
		},
		{
			name:          "restore log request with phase '' gets a url",
			key:           "heptio-ark/dr1",
			phase:         "",
			targetKind:    v1.DownloadTargetKindRestoreLog,
			targetName:    "backup1-20170912150214",
			restore:       arktest.NewTestRestore(v1.DefaultNamespace, "backup1-20170912150214", v1.RestorePhaseCompleted).WithBackup("backup1").Restore,
			expectedDir:   "backup1",
			expectedPhase: v1.DownloadRequestPhaseProcessed,
			expectedURL:   "signedURL",
		},
		{
			name:          "restore log request with phase New gets a url",
			key:           "heptio-ark/dr1",
			phase:         v1.DownloadRequestPhaseNew,
			targetKind:    v1.DownloadTargetKindRestoreLog,
			targetName:    "backup1-20170912150214",
			restore:       arktest.NewTestRestore(v1.DefaultNamespace, "backup1-20170912150214", v1.RestorePhaseCompleted).WithBackup("backup1").Restore,
			expectedDir:   "backup1",
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
				restoresInformer         = sharedInformers.Ark().V1().Restores()
				logger                   = arktest.NewLogger()
				clockTime, _             = time.Parse("Mon Jan 2 15:04:05 2006", "Mon Jan 2 15:04:05 2006")
			)

			c := NewDownloadRequestController(
				client.ArkV1(),
				downloadRequestsInformer,
				restoresInformer,
				nil, // objectStore
				"bucket",
				logger,
			).(*downloadRequestController)

			c.clock = clock.NewFakeClock(clockTime)

			var downloadRequest *v1.DownloadRequest

			if tc.expectedPhase == v1.DownloadRequestPhaseProcessed {
				expectedTarget := v1.DownloadTarget{
					Kind: tc.targetKind,
					Name: tc.targetName,
				}

				downloadRequest = &v1.DownloadRequest{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: v1.DefaultNamespace,
						Name:      "dr1",
					},
					Spec: v1.DownloadRequestSpec{
						Target: expectedTarget,
					},
				}
				downloadRequestsInformer.Informer().GetStore().Add(downloadRequest)

				if tc.restore != nil {
					restoresInformer.Informer().GetStore().Add(tc.restore)
				}

				c.createSignedURL = func(objectStore cloudprovider.ObjectStore, target v1.DownloadTarget, bucket, directory string, ttl time.Duration) (string, error) {
					require.Equal(t, expectedTarget, target)
					require.Equal(t, "bucket", bucket)
					require.Equal(t, tc.expectedDir, directory)
					require.Equal(t, 10*time.Minute, ttl)
					return "signedURL", nil
				}
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

			type PatchStatus struct {
				DownloadURL string                  `json:"downloadURL"`
				Phase       v1.DownloadRequestPhase `json:"phase"`
				Expiration  time.Time               `json:"expiration"`
			}

			type Patch struct {
				Status PatchStatus `json:"status"`
			}

			decode := func(decoder *json.Decoder) (interface{}, error) {
				actual := new(Patch)
				err := decoder.Decode(actual)

				return *actual, err
			}

			expected := Patch{
				Status: PatchStatus{
					DownloadURL: tc.expectedURL,
					Phase:       tc.expectedPhase,
					Expiration:  clockTime.Add(signedURLTTL),
				},
			}

			arktest.ValidatePatch(t, actions[0], expected, decode)
		})
	}
}
