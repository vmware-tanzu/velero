/*
Copyright the Velero Contributors.

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
	"strings"
	"testing"
	"time"

	flag "github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/vmware-tanzu/velero/pkg/client"
	clicmd "github.com/vmware-tanzu/velero/pkg/cmd"
	veleroflag "github.com/vmware-tanzu/velero/pkg/cmd/util/flag"
)

func TestBuildBackupStorageLocationSetsNamespace(t *testing.T) {
	o := NewCreateOptions()

	bsl, err := o.BuildBackupStorageLocation("velero-test-ns", false, false)
	assert.NoError(t, err)
	assert.Equal(t, "velero-test-ns", bsl.Namespace)
}

func TestBuildBackupStorageLocationSetsSyncPeriod(t *testing.T) {
	o := NewCreateOptions()
	o.BackupSyncPeriod = 2 * time.Minute

	bsl, err := o.BuildBackupStorageLocation("velero-test-ns", false, false)
	assert.NoError(t, err)
	assert.Nil(t, bsl.Spec.BackupSyncPeriod)

	bsl, err = o.BuildBackupStorageLocation("velero-test-ns", true, false)
	assert.NoError(t, err)
	assert.Equal(t, &metav1.Duration{Duration: 2 * time.Minute}, bsl.Spec.BackupSyncPeriod)
}

func TestBuildBackupStorageLocationSetsValidationFrequency(t *testing.T) {
	o := NewCreateOptions()
	o.ValidationFrequency = 2 * time.Minute

	bsl, err := o.BuildBackupStorageLocation("velero-test-ns", false, false)
	assert.NoError(t, err)
	assert.Nil(t, bsl.Spec.ValidationFrequency)

	bsl, err = o.BuildBackupStorageLocation("velero-test-ns", false, true)
	assert.NoError(t, err)
	assert.Equal(t, &metav1.Duration{Duration: 2 * time.Minute}, bsl.Spec.ValidationFrequency)
}

func TestBuildBackupStorageLocationSetsCredential(t *testing.T) {
	o := NewCreateOptions()

	bsl, err := o.BuildBackupStorageLocation("velero-test-ns", false, false)
	assert.NoError(t, err)
	assert.Nil(t, bsl.Spec.Credential)

	setErr := o.Credential.Set("my-secret=key-from-secret")
	assert.NoError(t, setErr)

	bsl, err = o.BuildBackupStorageLocation("velero-test-ns", false, true)
	assert.NoError(t, err)
	assert.Equal(t, &v1.SecretKeySelector{
		LocalObjectReference: v1.LocalObjectReference{Name: "my-secret"},
		Key:                  "key-from-secret",
	}, bsl.Spec.Credential)
}

func TestBuildBackupStorageLocationSetsLabels(t *testing.T) {
	o := NewCreateOptions()

	err := o.Labels.Set("key=value")
	assert.NoError(t, err)

	bsl, err := o.BuildBackupStorageLocation("velero-test-ns", false, false)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"key": "value"}, bsl.Labels)
}

func TestCreateCommand_Run(t *testing.T) {
	// create a config for factory
	baseName := "velero-bn"
	os.Setenv("VELERO_NAMESPACE", clicmd.VeleroNameSpace)
	config, err := client.LoadConfig()
	assert.Equal(t, err, nil)

	// create a factory
	f := client.NewFactory(baseName, config)
	cliFlags := new(flag.FlagSet)
	f.BindFlags(cliFlags)
	cliFlags.Parse(clicmd.FactoryFlags)

	// create command
	cmd := NewCreateCommand(f, "")
	assert.Equal(t, "Create a backup storage location", cmd.Short)

	// create a CreateOptions with full options set and then run this backup command
	name := "nameToBeCreated"
	provider := "aws"
	bucket := "velero123456"
	credential := veleroflag.NewMap()
	credential.Set("secret=a")

	defaultBackupStorageLocation := true
	prefix := "builds"
	backupSyncPeriod := "1m30s"
	validationFrequency := "128h1m6s"
	bslConfig := veleroflag.NewMap()
	bslConfigStr := "region=minio"
	bslConfig.Set(bslConfigStr)

	labels := "a=too,b=woo"
	caCertFile := "bsl-name-1"
	accessMode := "ReadWrite"

	flags := new(flag.FlagSet)
	o := NewCreateOptions()
	o.BindFlags(flags)

	flags.Parse([]string{"--provider", provider})
	flags.Parse([]string{"--bucket", bucket})
	flags.Parse([]string{"--credential", credential.String()})
	flags.Parse([]string{"--default"})
	flags.Parse([]string{"--prefix", prefix})
	flags.Parse([]string{"--backup-sync-period", backupSyncPeriod})
	flags.Parse([]string{"--validation-frequency", validationFrequency})
	flags.Parse([]string{"--config", bslConfigStr})
	flags.Parse([]string{"--labels", labels})
	flags.Parse([]string{"--cacert", caCertFile})
	flags.Parse([]string{"--access-mode", accessMode})

	args := []string{name, "arg2"}
	o.Complete(args, f)
	e := o.Validate(cmd, args, f)
	assert.Equal(t, e, nil)

	e = o.Run(cmd, f)
	assert.Contains(t, e.Error(), fmt.Sprintf("%s: no such file or directory", caCertFile))

	// verify all options are set as expected
	assert.Equal(t, name, o.Name)
	assert.Equal(t, provider, o.Provider)
	assert.Equal(t, bucket, o.Bucket)
	assert.Equal(t, true, reflect.DeepEqual(credential, o.Credential))
	assert.Equal(t, defaultBackupStorageLocation, o.DefaultBackupStorageLocation)
	assert.Equal(t, prefix, o.Prefix)
	assert.Equal(t, backupSyncPeriod, o.BackupSyncPeriod.String())
	assert.Equal(t, validationFrequency, o.ValidationFrequency.String())
	assert.Equal(t, true, reflect.DeepEqual(bslConfig, o.Config))
	assert.Equal(t, true, clicmd.CompareSlice(strings.Split(labels, ","), strings.Split(o.Labels.String(), ",")))
	assert.Equal(t, caCertFile, o.CACertFile)
	assert.Equal(t, accessMode, o.AccessMode.String())

	// create the other create command without fromSchedule option for Run() other branches
	cmd = NewCreateCommand(f, "velero backup-location create")
	assert.Equal(t, "Create a backup storage location", cmd.Short)

	o = NewCreateOptions()
	o.Labels.Set("velero.io/test=true")

	f.BindFlags(cliFlags)
	args = []string{"backup-name-2", "arg2"}
	o.Complete(args, f)

	e = o.Run(cmd, f)
	// Get failure of backup location resource creation
	assert.Contains(t, e.Error(), fmt.Sprintf("Get \"%s/api?timeout=", clicmd.HOST))

}

func TestCreateCommand_Execute(t *testing.T) {
	if os.Getenv(clicmd.TestExitFlag) == "1" {
		// create a config for factory
		baseName := "velero-bn"
		os.Setenv("VELERO_NAMESPACE", clicmd.VeleroNameSpace)
		config, err := client.LoadConfig()
		assert.Equal(t, err, nil)

		// create a factory
		f := client.NewFactory(baseName, config)
		cliFlags := new(flag.FlagSet)
		f.BindFlags(cliFlags)
		cliFlags.Parse(clicmd.FactoryFlags)

		// create command
		c := NewCreateCommand(f, "")
		c.SetArgs([]string{"bsl-1", "--provider=aws", "--bucket=bk1"})
		c.Execute()
		return
	}

	clicmd.TestProcessExit(t, exec.Command(os.Args[0], []string{"-test.run=TestCreateCommand_Execute"}...))
}
