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

package archive

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
)

func TestUnzipAndExtractBackup(t *testing.T) {
	tests := []struct {
		name      string
		files     []string
		IsTarball bool
		wantErr   bool
	}{
		{
			name:      "when the format of backup file is invalid, an error is returned",
			files:     []string{},
			IsTarball: false,
			wantErr:   true,
		},
		{
			name:      "when the backup tarball is empty, the function should work correctly and returns no error",
			files:     []string{},
			IsTarball: true,
			wantErr:   false,
		},
		{
			name: "when the backup tarball includes a mix of items, the function should work correctly and returns no error",
			files: []string{
				"root-dir/resources/namespace/cluster/example.json",
				"root-dir/resources/pods/namespaces/example.json",
				"root-dir/metadata/version",
			},
			IsTarball: true,
			wantErr:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ext := NewExtractor(test.NewLogger(), test.NewFakeFileSystem())
			var fileName string
			var err error
			if tc.IsTarball {
				fileName, err = createArchive(tc.files, ext.fs)
			} else {
				fileName, err = createRegular(ext.fs)
			}
			require.NoError(t, err)

			file, err := ext.fs.OpenFile(fileName, os.O_RDWR|os.O_CREATE, 0644)
			require.NoError(t, err)

			_, err = ext.UnzipAndExtractBackup(file.(io.Reader))
			if tc.wantErr && (err == nil) {
				t.Errorf("%s: wanted error but got nil", tc.name)
			}

			if !tc.wantErr && (err != nil) {
				t.Errorf("%s: wanted no error but got err: %v", tc.name, err)
			}
		})
	}
}

func createArchive(files []string, fs filesystem.Interface) (string, error) {
	outName := "output.tar.gz"
	out, err := fs.Create(outName)
	if err != nil {
		return outName, err
	}
	defer out.Close()
	gw := gzip.NewWriter(out)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Iterate over files and add them to the tar archive
	for _, file := range files {
		err := addToArchive(tw, file, fs)
		if err != nil {
			return outName, err
		}
	}

	return outName, nil
}

func addToArchive(tw *tar.Writer, filename string, fs filesystem.Interface) error {
	// Create the file
	file, err := fs.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Get FileInfo about size, mode, etc.
	info, err := fs.Stat(filename)
	if err != nil {
		return err
	}

	// Create a tar Header from the FileInfo data
	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return err
	}

	header.Name = filename
	err = tw.WriteHeader(header)
	if err != nil {
		return err
	}

	return nil
}

func createRegular(fs filesystem.Interface) (string, error) {
	outName := "output"
	out, err := fs.Create(outName)
	if err != nil {
		return outName, err
	}
	defer out.Close()

	return outName, nil
}
