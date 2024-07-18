package backup

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"testing"

	flag "github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vmware-tanzu/velero/pkg/builder"
	factorymocks "github.com/vmware-tanzu/velero/pkg/client/mocks"
	cmdtest "github.com/vmware-tanzu/velero/pkg/cmd/test"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	veleroexec "github.com/vmware-tanzu/velero/pkg/util/exec"
)

func TestNewDownloadCommand(t *testing.T) {
	// create a factory
	f := &factorymocks.Factory{}

	backupName := "backup-1"
	kbclient := velerotest.NewFakeControllerRuntimeClient(t)
	err := kbclient.Create(context.Background(), builder.ForBackup(cmdtest.VeleroNameSpace, backupName).Result())
	require.NoError(t, err)
	err = kbclient.Create(context.Background(), builder.ForBackup(cmdtest.VeleroNameSpace, "bk-to-be-download").Result())
	require.NoError(t, err)

	f.On("Namespace").Return(cmdtest.VeleroNameSpace)
	f.On("KubebuilderClient").Return(kbclient, nil)

	// Ensure the file does not exist before the test
	filePath := "bk-to-be-download-data.tar.gz"
	if _, err := os.Stat(filePath); err == nil {
		err := os.Remove(filePath)
		require.NoError(t, err)
	}

	// create command
	c := NewDownloadCommand(f)
	c.SetArgs([]string{"bk-to-be-download"})
	assert.Equal(t, "Download all Kubernetes manifests for a backup", c.Short)

	// create a DownloadOptions with full options set and then run this backup command
	output := "path/to/download/bk.json"
	force := true
	timeout := "1m30s"
	insecureSkipTlsVerify := false
	cacert := "secret=YHJKKS"

	flags := new(flag.FlagSet)
	o := NewDownloadOptions()
	o.BindFlags(flags)

	flags.Parse([]string{"--output", output})
	flags.Parse([]string{"--force"})
	flags.Parse([]string{"--timeout", timeout})
	flags.Parse([]string{fmt.Sprintf("--insecure-skip-tls-verify=%s", strconv.FormatBool(insecureSkipTlsVerify))})
	flags.Parse([]string{"--cacert", cacert})

	args := []string{backupName, "arg2"}

	e := o.Complete(args)
	assert.NoError(t, e)

	e = o.Validate(c, args, f)
	assert.NoError(t, e)

	// verify all options are set as expected
	assert.Equal(t, output, o.Output)
	assert.Equal(t, force, o.Force)
	assert.Equal(t, timeout, o.Timeout.String())
	assert.Equal(t, insecureSkipTlsVerify, o.InsecureSkipTLSVerify)
	assert.Equal(t, cacert, o.caCertFile)

	if os.Getenv(cmdtest.CaptureFlag) == "1" {
		e = c.Execute()
		defer os.Remove("bk-to-be-download-data.tar.gz")
		assert.NoError(t, e)
		return
	}

	// Prepare to simulate user input for confirmation prompt
	var input bytes.Buffer
	input.WriteString("Y\n")
	cmd := exec.Command(os.Args[0], []string{"-test.run=TestNewDownloadCommand"}...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=1", cmdtest.CaptureFlag))
	cmd.Stdin = &input // Provide the "Y" input for confirmation

	_, stderr, err := veleroexec.RunCommand(cmd)

	if err != nil {
		require.Contains(t, stderr, "download request download url timeout")
		return
	}
	t.Fatalf("process ran with err %v, want backup delete successfully", err)
}
