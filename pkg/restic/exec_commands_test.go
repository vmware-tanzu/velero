/*
Copyright 2019 the Velero contributors.

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

package restic

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
)

func Test_getSummaryLine(t *testing.T) {
	summaryLine := `{"message_type":"summary","files_new":0,"files_changed":0,"files_unmodified":3,"dirs_new":0,"dirs_changed":0,"dirs_unmodified":0,"data_blobs":0,"tree_blobs":0,"data_added":0,"total_files_processed":3,"total_bytes_processed":13238272000,"total_duration":0.319265105,"snapshot_id":"38515bb5"}`
	tests := []struct {
		name    string
		output  string
		wantErr bool
	}{
		{"no summary", `{"message_type":"status","percent_done":0,"total_files":1,"total_bytes":10485760000}
{"message_type":"status","percent_done":0,"total_files":3,"files_done":1,"total_bytes":13238272000}
`, true},
		{"no newline after summary", `{"message_type":"status","percent_done":0,"total_files":1,"total_bytes":10485760000}
{"message_type":"status","percent_done":0,"total_files":3,"files_done":1,"total_bytes":13238272000}
{"message_type":"summary","files_new":0,"files_changed":0,"files_unmodified":3,"dirs_new":0`, true},
		{"summary at end", `{"message_type":"status","percent_done":0,"total_files":1,"total_bytes":10485760000}
{"message_type":"status","percent_done":0,"total_files":3,"files_done":1,"total_bytes":13238272000}
{"message_type":"status","percent_done":1,"total_files":3,"files_done":3,"total_bytes":13238272000,"bytes_done":13238272000}
{"message_type":"summary","files_new":0,"files_changed":0,"files_unmodified":3,"dirs_new":0,"dirs_changed":0,"dirs_unmodified":0,"data_blobs":0,"tree_blobs":0,"data_added":0,"total_files_processed":3,"total_bytes_processed":13238272000,"total_duration":0.319265105,"snapshot_id":"38515bb5"}
`, false},
		{"summary before status", `{"message_type":"status","percent_done":0,"total_files":1,"total_bytes":10485760000}
{"message_type":"status","percent_done":0,"total_files":3,"files_done":1,"total_bytes":13238272000}
{"message_type":"summary","files_new":0,"files_changed":0,"files_unmodified":3,"dirs_new":0,"dirs_changed":0,"dirs_unmodified":0,"data_blobs":0,"tree_blobs":0,"data_added":0,"total_files_processed":3,"total_bytes_processed":13238272000,"total_duration":0.319265105,"snapshot_id":"38515bb5"}
{"message_type":"status","percent_done":1,"total_files":3,"files_done":3,"total_bytes":13238272000,"bytes_done":13238272000}
`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, err := getSummaryLine([]byte(tt.output))
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, summaryLine, string(summary))
			}
		})
	}
}

func Test_getLastLine(t *testing.T) {
	tests := []struct {
		output []byte
		want   string
	}{
		{[]byte(`last line
`), "last line"},
		{[]byte(`first line
second line
third line
`), "third line"},
		{[]byte(""), ""},
		{nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, []byte(tt.want), getLastLine([]byte(tt.output)))
		})
	}
}

func Test_getVolumeSize(t *testing.T) {
	files := map[string][]byte{
		"/file1.txt":              []byte("file1"),
		"/file2.txt":              []byte("file2"),
		"/file3.txt":              []byte("file3"),
		"/files/file4.txt":        []byte("file4"),
		"/files/nested/file5.txt": []byte("file5"),
	}
	fakefs := test.NewFakeFileSystem()

	var expectedSize int64
	for path, content := range files {
		fakefs.WithFile(path, content)
		expectedSize += int64(len(content))
	}

	fileSystem = fakefs
	defer func() { fileSystem = filesystem.NewFileSystem() }()

	actualSize, err := getVolumeSize("/")

	assert.NoError(t, err)
	assert.Equal(t, expectedSize, actualSize)
}
