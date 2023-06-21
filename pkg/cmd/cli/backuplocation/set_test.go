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

package backuplocation

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"testing"

	flag "github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	cmdtest "github.com/vmware-tanzu/velero/pkg/cmd/test"
	veleroflag "github.com/vmware-tanzu/velero/pkg/cmd/util/flag"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	factorymocks "github.com/vmware-tanzu/velero/pkg/client/mocks"
	versionedmocks "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/mocks"
	"github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/scheme"
	velerov1mocks "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/typed/velero/v1/mocks"
	veleroexec "github.com/vmware-tanzu/velero/pkg/util/exec"
)

func TestNewSetCommand(t *testing.T) {
	backupName := "arg2"
	// create a config for factory
	f := &factorymocks.Factory{}
	backups := &velerov1mocks.BackupInterface{}
	veleroV1 := &velerov1mocks.VeleroV1Interface{}
	client := &versionedmocks.Interface{}
	bk := &velerov1api.Backup{}
	kbclient := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()

	backups.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(bk, nil)
	veleroV1.On("Backups", mock.Anything).Return(backups, nil)
	client.On("VeleroV1").Return(veleroV1, nil)
	f.On("Client").Return(client, nil)
	f.On("Namespace").Return(mock.Anything)
	f.On("KubebuilderClient").Return(kbclient, nil)

	// create command
	c := NewSetCommand(f, "")
	assert.Equal(t, "Set specific features for a backup storage location", c.Short)

	// create a SetOptions with full options set and then run this backup command
	cacert := "a/b/c/ut-cert.ca"
	defaultBackupStorageLocation := true
	credential := veleroflag.NewMap()
	credential.Set("secret=a")

	flags := new(flag.FlagSet)
	o := NewSetOptions()
	o.BindFlags(flags)

	flags.Parse([]string{"--cacert", cacert})
	flags.Parse([]string{"--credential", credential.String()})
	flags.Parse([]string{"--default"})

	args := []string{backupName}
	o.Complete(args, f)
	e := o.Validate(c, args, f)
	assert.Equal(t, e, nil)

	e = o.Run(c, f)
	assert.Contains(t, e.Error(), fmt.Sprintf("%s: no such file or directory", cacert))

	// verify all options are set as expected
	assert.Equal(t, backupName, o.Name)
	assert.Equal(t, cacert, o.CACertFile)
	assert.Equal(t, defaultBackupStorageLocation, o.DefaultBackupStorageLocation)
	assert.Equal(t, true, reflect.DeepEqual(credential, o.Credential))

	assert.Contains(t, e.Error(), fmt.Sprintf("%s: no such file or directory", cacert))
}

func TestSetCommand_Execute(t *testing.T) {
	bsl := "bsl-1"
	if os.Getenv(cmdtest.CaptureFlag) == "1" {
		// create a config for factory
		f := &factorymocks.Factory{}
		backups := &velerov1mocks.BackupInterface{}
		veleroV1 := &velerov1mocks.VeleroV1Interface{}
		client := &versionedmocks.Interface{}
		bk := &velerov1api.Backup{}
		kbclient := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()

		backups.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(bk, nil)
		veleroV1.On("Backups", mock.Anything).Return(backups, nil)
		client.On("VeleroV1").Return(veleroV1, nil)
		f.On("Client").Return(client, nil)
		f.On("Namespace").Return(mock.Anything)
		f.On("KubebuilderClient").Return(kbclient, nil)

		// create command
		c := NewSetCommand(f, "velero backup-location set")
		c.SetArgs([]string{bsl})
		c.Execute()
		return
	}

	cmd := exec.Command(os.Args[0], []string{"-test.run=TestSetCommand_Execute"}...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=1", cmdtest.CaptureFlag))
	_, stderr, err := veleroexec.RunCommand(cmd)

	if err != nil {
		assert.Contains(t, stderr, fmt.Sprintf("backupstoragelocations.velero.io \"%s\" not found", bsl))
		return
	}
	t.Fatalf("process ran with err %v, want backup delete successfully", err)
}
