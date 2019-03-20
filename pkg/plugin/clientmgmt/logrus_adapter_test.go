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
package clientmgmt

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestArgsToFields(t *testing.T) {
	tests := []struct {
		name           string
		args           []interface{}
		expectedFields logrus.Fields
	}{
		{
			name:           "empty args results in empty map of fields",
			args:           []interface{}{},
			expectedFields: logrus.Fields(map[string]interface{}{}),
		},
		{
			name: "matching string keys/values are correctly set as fields",
			args: []interface{}{"key-1", "value-1", "key-2", "value-2"},
			expectedFields: logrus.Fields(map[string]interface{}{
				"key-1": "value-1",
				"key-2": "value-2",
			}),
		},
		{
			name: "time/timestamp/level entries are removed",
			args: []interface{}{"time", time.Now(), "key-1", "value-1", "timestamp", time.Now(), "key-2", "value-2", "level", "WARN"},
			expectedFields: logrus.Fields(map[string]interface{}{
				"key-1": "value-1",
				"key-2": "value-2",
			}),
		},
		{
			name: "odd number of args adds the last arg as a field with a nil value",
			args: []interface{}{"key-1", "value-1", "key-2", "value-2", "key-3"},
			expectedFields: logrus.Fields(map[string]interface{}{
				"key-1": "value-1",
				"key-2": "value-2",
				"key-3": nil,
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expectedFields, argsToFields(test.args...))
		})
	}
}
