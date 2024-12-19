// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeLocalTestDir(t *testing.T, dir string) {
	t.Logf("creating local test dir: %s", dir)
	if err := os.MkdirAll(dir, 0744); err != nil && !os.IsExist(err) {
		t.Fatalf("makeLocalTestDir: failed to create dir: %s", err)
	}
	t.Logf("dir created: %s", dir)
}

func MakeLocalTestFile(t *testing.T, filePath, content string) {
	srcDir := filepath.Dir(filePath)
	if len(srcDir) > 0 && srcDir != "." {
		makeLocalTestDir(t, srcDir)
	}

	t.Logf("creating local test file: %s", filePath)
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("MakeLocalTestFile: failed to create file: %s", err)
	}
	defer file.Close()
	buf := bytes.NewBufferString(content)
	if _, err := buf.WriteTo(file); err != nil {
		t.Fatal(err)
	}
	t.Logf("local test file created: %s", file.Name())
}

func RemoveLocalTestFile(t *testing.T, fileName string) {
	t.Logf("removing local test path: %s", fileName)
	if err := os.RemoveAll(fileName); err != nil && !os.IsNotExist(err) {
		t.Fatalf("RemoveLocalTestFile: failed: %s", err)
	}
}

func makeRemoteTestSSHDir(t *testing.T, args SSHArgs, dir string) {
	t.Logf("creating remote test dir over SSH: %s", dir)
	_, err := Run(args, nil, fmt.Sprintf(`mkdir -p %s`, dir))
	if err != nil {
		t.Fatalf("makeRemoteTestSSHDir: failed: %s", err)
	}
	// validate
	result, err := Run(args, nil, fmt.Sprintf(`stat %s`, dir))
	if err != nil {
		t.Fatalf("makeRemoteTestSSHDir %s", err)
	}
	t.Logf("dir created: %s", result)
}

func MakeRemoteTestSSHFile(t *testing.T, args SSHArgs, filePath, content string) {
	MakeRemoteTestSSHDir(t, args, filePath)

	t.Logf("creating test file over SSH: %s", filePath)
	_, err := Run(args, nil, fmt.Sprintf(`echo '%s' > %s`, content, filePath))
	if err != nil {
		t.Fatalf("MakeRemoteTestSSHFile: failed: %s", err)
	}

	result, _ := Run(args, nil, fmt.Sprintf(`ls %s`, filePath))
	t.Logf("file created: %s", result)
}

func MakeRemoteTestSSHDir(t *testing.T, args SSHArgs, filePath string) {
	dir := filepath.Dir(filePath)
	if len(dir) > 0 && dir != "." {
		makeRemoteTestSSHDir(t, args, dir)
	}
}

func AssertRemoteTestSSHFile(t *testing.T, args SSHArgs, filePath string) {
	t.Logf("stat remote SSH test file: %s", filePath)
	_, err := Run(args, nil, fmt.Sprintf(`stat %s`, filePath))
	if err != nil {
		t.Fatal(err)
	}
}

func RemoveRemoteTestSSHFile(t *testing.T, args SSHArgs, fileName string) {
	t.Logf("removing test file over SSH: %s", fileName)
	_, err := Run(args, nil, fmt.Sprintf(`rm -rf %s`, fileName))
	if err != nil {
		t.Fatal(err)
	}
}

func getTestFileContent(t *testing.T, fileName string) string {
	file, err := os.Open(fileName)
	if err != nil {
		t.Fatal(err)
	}
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(file); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(buf.String())
}
