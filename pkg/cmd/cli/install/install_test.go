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
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	factorymocks "github.com/vmware-tanzu/velero/pkg/client/mocks"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
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

// makeValidateCmd returns a minimal *cobra.Command that satisfies output.ValidateFlags.
func makeValidateCmd() *cobra.Command {
	c := &cobra.Command{}
	// output.ValidateFlags only inspects the "output" flag; add it so validation passes.
	c.Flags().StringP("output", "o", "", "output format")
	return c
}

// configMapInNamespace builds a ConfigMap with a single JSON data entry in the given namespace.
func configMapInNamespace(namespace, name, jsonValue string) *corev1api.ConfigMap {
	return &corev1api.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Data: map[string]string{
			"config": jsonValue,
		},
	}
}

// TestValidateConfigMapsUseFactoryNamespace verifies that Validate resolves the target
// namespace via f.Namespace() (not o.Namespace) for all three ConfigMap flags.
//
// Before the fix, o.Namespace was "" at Validate time (Complete had not run yet),
// so VerifyJSONConfigs queried the "default" namespace instead of the intended one.
func TestValidateConfigMapsUseFactoryNamespace(t *testing.T) {
	const targetNS = "tenant-b"
	const defaultNS = "default"

	// Shared options that satisfy every other validation gate:
	//   - NoDefaultBackupLocation=true + UseVolumeSnapshots=false skips provider/bucket/plugins checks
	//   - NoSecret=true satisfies the secret-file check
	//   - o.Namespace is intentionally left as "" to mirror the pre-Complete state
	baseOptions := func() *Options {
		o := NewInstallOptions()
		o.NoDefaultBackupLocation = true
		o.UseVolumeSnapshots = false
		o.NoSecret = true
		return o
	}

	tests := []struct {
		name       string
		setupOpts  func(o *Options, cmName string)
		cmJSON     string
		wantErrMsg string // substring expected in error; empty means success
	}{
		{
			name: "NodeAgentConfigMap found in factory namespace",
			setupOpts: func(o *Options, cmName string) {
				o.NodeAgentConfigMap = cmName
			},
			cmJSON: `{}`,
		},
		{
			name: "NodeAgentConfigMap not found when only in default namespace",
			setupOpts: func(o *Options, cmName string) {
				o.NodeAgentConfigMap = cmName
			},
			cmJSON:     `{}`,
			wantErrMsg: "--node-agent-configmap specified ConfigMap",
		},
		{
			name: "RepoMaintenanceJobConfigMap found in factory namespace",
			setupOpts: func(o *Options, cmName string) {
				o.RepoMaintenanceJobConfigMap = cmName
			},
			cmJSON: `{}`,
		},
		{
			name: "RepoMaintenanceJobConfigMap not found when only in default namespace",
			setupOpts: func(o *Options, cmName string) {
				o.RepoMaintenanceJobConfigMap = cmName
			},
			cmJSON:     `{}`,
			wantErrMsg: "--repo-maintenance-job-configmap specified ConfigMap",
		},
		{
			name: "BackupRepoConfigMap found in factory namespace",
			setupOpts: func(o *Options, cmName string) {
				o.BackupRepoConfigMap = cmName
			},
			cmJSON: `{}`,
		},
		{
			name: "BackupRepoConfigMap not found when only in default namespace",
			setupOpts: func(o *Options, cmName string) {
				o.BackupRepoConfigMap = cmName
			},
			cmJSON:     `{}`,
			wantErrMsg: "--backup-repository-configmap specified ConfigMap",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			const cmName = "my-config"

			// Decide where to place the ConfigMap:
			// "not found" cases put it in "default", so the factory namespace lookup misses it.
			cmNamespace := targetNS
			if tc.wantErrMsg != "" {
				cmNamespace = defaultNS
			}

			cm := configMapInNamespace(cmNamespace, cmName, tc.cmJSON)
			kbClient := velerotest.NewFakeControllerRuntimeClient(t, cm)

			f := &factorymocks.Factory{}
			f.On("Namespace").Return(targetNS)
			f.On("KubebuilderClient").Return(kbClient, nil)

			o := baseOptions()
			tc.setupOpts(o, cmName)

			c := makeValidateCmd()
			c.SetContext(context.Background())

			err := o.Validate(c, []string{}, f)

			if tc.wantErrMsg == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrMsg)
			}
		})
	}
}
