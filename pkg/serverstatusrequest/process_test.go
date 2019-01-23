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

package serverstatusrequest

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/clock"

	arkv1api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/buildinfo"
	"github.com/heptio/ark/pkg/generated/clientset/versioned/fake"
)

func statusRequestBuilder() *Builder {
	return NewBuilder().Namespace(arkv1api.DefaultNamespace).Name("sr-1")
}

func TestProcess(t *testing.T) {
	// now will be used to set the fake clock's time; capture
	// it here so it can be referenced in the test case defs.
	now, err := time.Parse(time.RFC1123, time.RFC1123)
	require.NoError(t, err)
	now = now.Local()

	buildinfo.Version = "test-version-val"

	tests := []struct {
		name           string
		req            *arkv1api.ServerStatusRequest
		expected       *arkv1api.ServerStatusRequest
		expectedErrMsg string
	}{
		{
			name: "server status request with empty phase gets processed",
			req:  statusRequestBuilder().Build(),
			expected: statusRequestBuilder().
				ServerVersion(buildinfo.Version).
				Phase(arkv1api.ServerStatusRequestPhaseProcessed).
				ProcessedTimestamp(now).
				Build(),
		},
		{
			name: "server status request with phase=New gets processed",
			req: statusRequestBuilder().
				Phase(arkv1api.ServerStatusRequestPhaseNew).
				Build(),
			expected: statusRequestBuilder().
				ServerVersion(buildinfo.Version).
				Phase(arkv1api.ServerStatusRequestPhaseProcessed).
				ProcessedTimestamp(now).
				Build(),
		},
		{
			name: "server status request with phase=Processed gets deleted if expired",
			req: statusRequestBuilder().
				Phase(arkv1api.ServerStatusRequestPhaseProcessed).
				ProcessedTimestamp(now.Add(-61 * time.Second)).
				Build(),
			expected: nil,
		},
		{
			name: "server status request with phase=Processed does not get deleted if not expired",
			req: statusRequestBuilder().
				Phase(arkv1api.ServerStatusRequestPhaseProcessed).
				ProcessedTimestamp(now.Add(-59 * time.Second)).
				Build(),
			expected: statusRequestBuilder().
				Phase(arkv1api.ServerStatusRequestPhaseProcessed).
				ProcessedTimestamp(now.Add(-59 * time.Second)).
				Build(),
		},
		{
			name: "server status request with invalid phase returns an error",
			req: statusRequestBuilder().
				Phase(arkv1api.ServerStatusRequestPhase("an-invalid-phase")).
				Build(),
			expected: statusRequestBuilder().
				Phase(arkv1api.ServerStatusRequestPhase("an-invalid-phase")).
				Build(),
			expectedErrMsg: "unexpected ServerStatusRequest phase \"an-invalid-phase\"",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := fake.NewSimpleClientset(tc.req)

			err := Process(tc.req, client.ArkV1(), clock.NewFakeClock(now), logrus.StandardLogger())
			if tc.expectedErrMsg == "" {
				assert.Nil(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErrMsg)
			}

			res, err := client.ArkV1().ServerStatusRequests(tc.req.Namespace).Get(tc.req.Name, metav1.GetOptions{})
			if tc.expected == nil {
				assert.Nil(t, res)
				assert.True(t, apierrors.IsNotFound(err))
			} else {
				assert.Equal(t, tc.expected, res)
				assert.Nil(t, err)
			}
		})
	}
}
