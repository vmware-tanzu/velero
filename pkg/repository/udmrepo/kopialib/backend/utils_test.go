/*
Copyright the Velero contributors.

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

package backend

import (
	"context"
	"fmt"
	"testing"

	"github.com/kopia/kopia/repo/logging"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	storagemocks "github.com/vmware-tanzu/velero/pkg/repository/udmrepo/kopialib/backend/mocks"
)

func TestOptionalHaveBool(t *testing.T) {
	var expectMsg string
	testCases := []struct {
		name          string
		key           string
		flags         map[string]string
		logger        *storagemocks.Logger
		retFuncErrorf func(mock.Arguments)
		expectMsg     string
		retValue      bool
	}{
		{
			name:     "key not exist",
			key:      "fake-key",
			flags:    map[string]string{},
			retValue: false,
		},
		{
			name: "value valid",
			key:  "fake-key",
			flags: map[string]string{
				"fake-key": "true",
			},
			retValue: true,
		},
		{
			name: "value invalid",
			key:  "fake-key",
			flags: map[string]string{
				"fake-key": "fake-value",
			},
			logger: new(storagemocks.Logger),
			retFuncErrorf: func(args mock.Arguments) {
				expectMsg = fmt.Sprintf(args[0].(string), args[1].(string), args[2].(string), args[3].(error))
			},
			expectMsg: "Ignore fake-key, value [fake-value] is invalid, err strconv.ParseBool: parsing \"fake-value\": invalid syntax",
			retValue:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.logger != nil {
				tc.logger.On("Errorf", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(tc.retFuncErrorf)
			}

			ctx := logging.WithLogger(context.Background(), func(module string) logging.Logger {
				return tc.logger
			})

			retValue := optionalHaveBool(ctx, tc.key, tc.flags)

			require.Equal(t, retValue, tc.retValue)
			require.Equal(t, tc.expectMsg, expectMsg)
		})
	}
}
