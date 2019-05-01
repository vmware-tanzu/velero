/*
Copyright 2019 the Velero contributors.

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

package label

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetValidLabelName(t *testing.T) {
	tests := []struct {
		name          string
		label         string
		expectedLabel string
	}{
		{
			name:          "valid label name should not be modified",
			label:         "short label value",
			expectedLabel: "short label value",
		},
		{
			name:          "label with more than 63 characters should be modified",
			label:         "this_is_a_very_long_label_value_that_will_be_rejected_by_Kubernetes",
			expectedLabel: "this_is_a_very_long_label_value_that_will_be_rejected_by_8d0722",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			labelVal := GetValidName(test.label)
			assert.Equal(t, test.expectedLabel, labelVal)
		})
	}
}
