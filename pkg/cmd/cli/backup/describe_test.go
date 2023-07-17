/*
Copyright The Velero Contributors.

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

package backup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
	controllerclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/pkg/builder"
	factorymocks "github.com/vmware-tanzu/velero/pkg/client/mocks"
	cmdtest "github.com/vmware-tanzu/velero/pkg/cmd/test"
	"github.com/vmware-tanzu/velero/pkg/features"
	"github.com/vmware-tanzu/velero/pkg/test"
	veleroexec "github.com/vmware-tanzu/velero/pkg/util/exec"
)

func TestNewDescribeCommand(t *testing.T) {
	// create a factory
	f := &factorymocks.Factory{}
	backupName := "bk-describe-1"
	testBackup := builder.ForBackup(cmdtest.VeleroNameSpace, backupName).Result()

	clientConfig := rest.Config{}
	kbClient := test.NewFakeControllerRuntimeClient(t)
	kbClient.Create(context.Background(), testBackup, &controllerclient.CreateOptions{})

	f.On("ClientConfig").Return(&clientConfig, nil)
	f.On("Namespace").Return(cmdtest.VeleroNameSpace)
	f.On("KubebuilderClient").Return(kbClient, nil)

	// create command
	c := NewDescribeCommand(f, "velero backup describe")
	assert.Equal(t, "Describe backups", c.Short)

	features.NewFeatureFlagSet("EnableCSI")
	defer features.NewFeatureFlagSet()

	c.SetArgs([]string{backupName})
	e := c.Execute()
	assert.NoError(t, e)

	if os.Getenv(cmdtest.CaptureFlag) == "1" {
		return
	}
	cmd := exec.Command(os.Args[0], []string{"-test.run=TestNewDescribeCommand"}...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=1", cmdtest.CaptureFlag))
	stdout, _, err := veleroexec.RunCommand(cmd)

	if err == nil {
		assert.Contains(t, stdout, "Velero-Native Snapshots: <none included>")
		assert.Contains(t, stdout, fmt.Sprintf("Name:         %s", backupName))
		return
	}
	t.Fatalf("process ran with err %v, want backups by get()", err)
}
