/*
Copyright 2018 the Velero contributors.

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
package process

import (
	"io/fs"
	"os"
	"sort"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vmware-tanzu/velero/pkg/test"
)

func TestNewRegistry(t *testing.T) {
	logger := test.NewLogger()
	logLevel := logrus.InfoLevel
	dir := "/plugins"

	r := NewRegistry(dir, logger, logLevel).(*registry)
	assert.Equal(t, dir, r.dir)
	assert.Equal(t, logger, r.logger)
	assert.Equal(t, logLevel, r.logLevel)
	assert.NotNil(t, r.pluginsByID)
	assert.Empty(t, r.pluginsByID)
	assert.NotNil(t, r.pluginsByKind)
	assert.Empty(t, r.pluginsByKind)
}

type fakeFileInfo struct {
	fs.FileInfo
	name string
	mode os.FileMode
}

func (f *fakeFileInfo) Mode() os.FileMode {
	return f.mode
}

func (f *fakeFileInfo) Name() string {
	return f.name
}

func TestExecutable(t *testing.T) {
	tests := []struct {
		name             string
		fileName         string
		mode             uint32
		expectExecutable bool
	}{
		{
			name: "no perms",
			mode: 0o000,
		},
		{
			name: "r--r--r--",
			mode: 0o444,
		},
		{
			name: "rw-rw-rw-",
			mode: 0o666,
		},
		{
			name:             "--x------",
			mode:             0o100,
			expectExecutable: true,
		},
		{
			name:             "-----x---",
			mode:             0o010,
			expectExecutable: true,
		},
		{
			name:             "--------x",
			mode:             0o001,
			expectExecutable: true,
		},
		{
			name:             "rwxrwxrwx",
			mode:             0o777,
			expectExecutable: true,
		},
		{
			name:             "windows lower case",
			fileName:         "test.exe",
			mode:             0,
			expectExecutable: true,
		},
		{
			name:             "windows upper case",
			fileName:         "test.EXE",
			mode:             0,
			expectExecutable: true,
		},
		{
			name:     "windows wrong ext",
			fileName: "test.EXE1",
			mode:     0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info := &fakeFileInfo{
				name: test.fileName,
				mode: os.FileMode(test.mode),
			}

			assert.Equal(t, test.expectExecutable, executable(info))
		})
	}
}

func TestReadPluginsDir(t *testing.T) {
	logger := test.NewLogger()
	logLevel := logrus.InfoLevel
	dir := "/plugins"

	r := NewRegistry(dir, logger, logLevel).(*registry)
	r.fs = test.NewFakeFileSystem().
		WithFileAndMode("/plugins/executable1", []byte("plugin1"), 0o755).
		WithFileAndMode("/plugins/nonexecutable2", []byte("plugin2"), 0o644).
		WithFileAndMode("/plugins/executable3", []byte("plugin3"), 0o755).
		WithFileAndMode("/plugins/nested/executable4", []byte("plugin4"), 0o755).
		WithFileAndMode("/plugins/nested/nonexecutable5", []byte("plugin4"), 0o644).
		WithFileAndMode("/plugins/nested/win-exe1.exe", []byte("plugin4"), 0o600).
		WithFileAndMode("/plugins/nested/WIN-EXE2.EXE", []byte("plugin4"), 0o600)

	plugins, err := r.readPluginsDir(dir)
	require.NoError(t, err)

	expected := []string{
		"/plugins/executable1",
		"/plugins/executable3",
		"/plugins/nested/executable4",
		"/plugins/nested/win-exe1.exe",
		"/plugins/nested/WIN-EXE2.EXE",
	}

	sort.Strings(plugins)
	sort.Strings(expected)
	assert.Equal(t, expected, plugins)
}
