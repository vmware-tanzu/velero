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
	"github.com/stretchr/testify/require"
)

func TestPriorityClassNameFlag(t *testing.T) {
	// Test that the flag is properly defined
	o := NewInstallOptions()
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	o.BindFlags(flags)

	// Verify the server priority class flag exists
	serverFlag := flags.Lookup("server-priority-class-name")
	assert.NotNil(t, serverFlag, "server-priority-class-name flag should exist")
	assert.Equal(t, "Priority class name for the Velero server deployment. Optional.", serverFlag.Usage)

	// Verify the node agent priority class flag exists
	nodeAgentFlag := flags.Lookup("node-agent-priority-class-name")
	assert.NotNil(t, nodeAgentFlag, "node-agent-priority-class-name flag should exist")
	assert.Equal(t, "Priority class name for the node agent daemonset. Optional.", nodeAgentFlag.Usage)

	// Test with values for both server and node agent
	testCases := []struct {
		name                       string
		serverPriorityClassName    string
		nodeAgentPriorityClassName string
		expectedServerValue        string
		expectedNodeAgentValue     string
	}{
		{
			name:                       "with both priority class names",
			serverPriorityClassName:    "high-priority",
			nodeAgentPriorityClassName: "medium-priority",
			expectedServerValue:        "high-priority",
			expectedNodeAgentValue:     "medium-priority",
		},
		{
			name:                       "with only server priority class name",
			serverPriorityClassName:    "high-priority",
			nodeAgentPriorityClassName: "",
			expectedServerValue:        "high-priority",
			expectedNodeAgentValue:     "",
		},
		{
			name:                       "with only node agent priority class name",
			serverPriorityClassName:    "",
			nodeAgentPriorityClassName: "medium-priority",
			expectedServerValue:        "",
			expectedNodeAgentValue:     "medium-priority",
		},
		{
			name:                       "without priority class names",
			serverPriorityClassName:    "",
			nodeAgentPriorityClassName: "",
			expectedServerValue:        "",
			expectedNodeAgentValue:     "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			o := NewInstallOptions()
			o.ServerPriorityClassName = tc.serverPriorityClassName
			o.NodeAgentPriorityClassName = tc.nodeAgentPriorityClassName

			veleroOptions, err := o.AsVeleroOptions()
			require.NoError(t, err)
			assert.Equal(t, tc.expectedServerValue, veleroOptions.ServerPriorityClassName)
			assert.Equal(t, tc.expectedNodeAgentValue, veleroOptions.NodeAgentPriorityClassName)
		})
	}
}
