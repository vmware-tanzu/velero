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
	"testing"
	"time"

	testlogger "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	core "k8s.io/client-go/testing"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/generated/clientset/fake"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions"
	"github.com/heptio/ark/pkg/util/test"
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
				backupService            = &test.BackupService{}
				logger, _                = testlogger.NewNullLogger()
			)
			defer backupService.AssertExpectations(t)

			c := NewDownloadRequestController(
				client.ArkV1(),
				downloadRequestsInformer,
				backupService,
				"bucket",
				logger,
			).(*downloadRequestController)

			if tc.expectedPhase == v1.DownloadRequestPhaseProcessed {
				target := v1.DownloadTarget{
					Kind: tc.targetKind,
					Name: tc.targetName,
				}

				downloadRequestsInformer.Informer().GetStore().Add(
					&v1.DownloadRequest{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: v1.DefaultNamespace,
							Name:      "dr1",
						},
						Spec: v1.DownloadRequestSpec{
							Target: target,
						},
					},
				)

				backupService.On("CreateSignedURL", target, "bucket", 10*time.Minute).Return("signedURL", nil)
			}

			var updatedRequest *v1.DownloadRequest

			client.PrependReactor("update", "downloadrequests", func(action core.Action) (bool, runtime.Object, error) {
				obj := action.(core.UpdateAction).GetObject()
				r, ok := obj.(*v1.DownloadRequest)
				require.True(t, ok)
				updatedRequest = r
				return true, obj, nil
			})

			// method under test
			err := c.processDownloadRequest(tc.key)

			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)

			var (
				updatedPhase v1.DownloadRequestPhase
				updatedURL   string
			)
			if updatedRequest != nil {
				updatedPhase = updatedRequest.Status.Phase
				updatedURL = updatedRequest.Status.DownloadURL
			}
			assert.Equal(t, tc.expectedPhase, updatedPhase)
			assert.Equal(t, tc.expectedURL, updatedURL)
		})
	}
}
