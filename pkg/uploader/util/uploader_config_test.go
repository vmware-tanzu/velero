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

package util

import (
	"reflect"
	"testing"

	"github.com/pkg/errors"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

func TestStoreBackupConfig(t *testing.T) {
	config := &velerov1api.UploaderConfigForBackup{
		ParallelFilesUpload: 3,
	}

	expectedData := map[string]string{
		parallelFilesUpload: "3",
	}

	result := StoreBackupConfig(config)

	if !reflect.DeepEqual(result, expectedData) {
		t.Errorf("Expected: %v, but got: %v", expectedData, result)
	}
}

func TestStoreRestoreConfig(t *testing.T) {
	boolTrue := true
	config := &velerov1api.UploaderConfigForRestore{
		WriteSparseFiles: &boolTrue,
	}

	expectedData := map[string]string{
		writeSparseFiles: "true",
	}

	result := StoreRestoreConfig(config)

	if !reflect.DeepEqual(result, expectedData) {
		t.Errorf("Expected: %v, but got: %v", expectedData, result)
	}
}

func TestGetParallelFilesUpload(t *testing.T) {
	tests := []struct {
		name           string
		uploaderCfg    map[string]string
		expectedResult int
		expectedError  error
	}{
		{
			name:           "Valid ParallelFilesUpload",
			uploaderCfg:    map[string]string{parallelFilesUpload: "5"},
			expectedResult: 5,
			expectedError:  nil,
		},
		{
			name:           "Missing ParallelFilesUpload",
			uploaderCfg:    map[string]string{},
			expectedResult: 0,
			expectedError:  nil,
		},
		{
			name:           "Invalid ParallelFilesUpload (not a number)",
			uploaderCfg:    map[string]string{parallelFilesUpload: "invalid"},
			expectedResult: 0,
			expectedError:  errors.Wrap(errors.New("strconv.Atoi: parsing \"invalid\": invalid syntax"), "failed to parse ParallelFilesUpload config"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := GetParallelFilesUpload(test.uploaderCfg)

			if result != test.expectedResult {
				t.Errorf("Expected result %d, but got %d", test.expectedResult, result)
			}

			if (err == nil && test.expectedError != nil) || (err != nil && test.expectedError == nil) || (err != nil && test.expectedError != nil && err.Error() != test.expectedError.Error()) {
				t.Errorf("Expected error '%v', but got '%v'", test.expectedError, err)
			}
		})
	}
}

func TestGetWriteSparseFiles(t *testing.T) {
	tests := []struct {
		name           string
		uploaderCfg    map[string]string
		expectedResult bool
		expectedError  error
	}{
		{
			name:           "Valid WriteSparseFiles (true)",
			uploaderCfg:    map[string]string{writeSparseFiles: "true"},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name:           "Valid WriteSparseFiles (false)",
			uploaderCfg:    map[string]string{writeSparseFiles: "false"},
			expectedResult: false,
			expectedError:  nil,
		},
		{
			name:           "Invalid WriteSparseFiles (not a boolean)",
			uploaderCfg:    map[string]string{writeSparseFiles: "invalid"},
			expectedResult: false,
			expectedError:  errors.Wrap(errors.New("strconv.ParseBool: parsing \"invalid\": invalid syntax"), "failed to parse WriteSparseFiles config"),
		},
		{
			name:           "Missing WriteSparseFiles",
			uploaderCfg:    map[string]string{},
			expectedResult: false,
			expectedError:  nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := GetWriteSparseFiles(test.uploaderCfg)

			if result != test.expectedResult {
				t.Errorf("Expected result %t, but got %t", test.expectedResult, result)
			}

			if (err == nil && test.expectedError != nil) || (err != nil && test.expectedError == nil) || (err != nil && test.expectedError != nil && err.Error() != test.expectedError.Error()) {
				t.Errorf("Expected error '%v', but got '%v'", test.expectedError, err)
			}
		})
	}
}
