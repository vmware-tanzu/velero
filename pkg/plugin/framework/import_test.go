package framework

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// test that this framework do not import cloud provider

// Prevent https://github.com/vmware-tanzu/velero/issues/8207 and https://github.com/vmware-tanzu/velero/issues/8157
func TestPkgImportNoCloudProvider(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("No caller information")
	}
	t.Logf("Current test file path: %s", filename)
	t.Logf("Current test directory: %s", filepath.Dir(filename)) // should be "pkg/install"
	// go list -f {{.Deps}} ./pkg/install/
	cmd := exec.Command(
		"go",
		"list",
		"-f",
		"{{.Deps}}",
		".",
	)
	// set cmd.Dir to this package even if executed from different dir
	cmd.Dir = filepath.Dir(filename)
	output, err := cmd.Output()
	require.NoError(t, err)
	// split dep by line, replace space with newline
	deps := strings.ReplaceAll(string(output), " ", "\n")
	require.NotEmpty(t, deps)
	// ignore k8s.io
	k8sio, err := regexp.Compile("^k8s.io")
	require.NoError(t, err)
	cloudProvider, err := regexp.Compile("aws|gcp|azure")
	require.NoError(t, err)
	// depsArr :=[]string{}
	cloudProviderDeps := []string{}
	for _, dep := range strings.Split(deps, "\n") {
		if !k8sio.MatchString(dep) {
			// depsArr = append(depsArr, dep)
			if cloudProvider.MatchString(dep) {
				cloudProviderDeps = append(cloudProviderDeps, dep)
			}
		}
	}
	require.Empty(t, cloudProviderDeps)
}
