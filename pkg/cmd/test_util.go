package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
)

var TRUE string = "true"
var VeleroNameSpace string = "velero-test"
var HOST string = "https://horse.org:4443"
var TestExitFlag string = "BE_EXECUTE"
var FactoryFlags []string = []string{"--kubeconfig", "../../../test/kubeconfig", "--kubecontext", "federal-context"}

func CompareSlice(x, y []string) bool {
	less := func(a, b string) bool { return a < b }
	return cmp.Diff(x, y, cmpopts.SortSlices(less)) == ""
}

func TestProcessExit(t *testing.T, cmd *exec.Cmd) {
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=1", TestExitFlag))
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		assert.Equal(t, "exit status 1", e.Error())
		return
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}
