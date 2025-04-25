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

package install

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func TestPriorityClassNameFlag(t *testing.T) {
	// Test that the flag is properly defined
	o := NewInstallOptions()
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	o.BindFlags(flags)

	// Verify the flag exists
	flag := flags.Lookup("priority-class-name")
	assert.NotNil(t, flag, "priority-class-name flag should exist")
	assert.Equal(t, "Priority class name for the Velero deployment, node agent daemonset, and maintenance jobs. Optional.", flag.Usage)

	// Test with a value
	testCases := []struct {
		name              string
		priorityClassName string
		expectedValue     string
	}{
		{
			name:              "with priority class name",
			priorityClassName: "high-priority",
			expectedValue:     "high-priority",
		},
		{
			name:              "without priority class name",
			priorityClassName: "",
			expectedValue:     "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			o := NewInstallOptions()
			o.PriorityClassName = tc.priorityClassName

			veleroOptions, err := o.AsVeleroOptions()
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedValue, veleroOptions.PriorityClassName)
		})
	}
}
