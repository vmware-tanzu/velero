package uploaderconfig

import (
	"reflect"
	"strings"
	"testing"

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

	if !reflect.DeepEqual(*result, expectedData) {
		t.Errorf("Expected: %v, but got: %v", expectedData, *result)
	}
}

func TestStoreRestoreConfig(t *testing.T) {
	config := &velerov1api.UploaderConfigForRestore{
		WriteSparseFiles: true,
	}

	expectedData := map[string]string{
		writeSparseFiles: "true",
	}

	result := StoreRestoreConfig(config)

	if !reflect.DeepEqual(*result, expectedData) {
		t.Errorf("Expected: %v, but got: %v", expectedData, *result)
	}
}

func TestGetBackupConfig(t *testing.T) {
	data := &map[string]string{
		parallelFilesUpload: "3",
	}

	expectedConfig := velerov1api.UploaderConfigForBackup{
		ParallelFilesUpload: 3,
	}

	result, err := GetBackupConfig(data)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !reflect.DeepEqual(result, expectedConfig) {
		t.Errorf("Expected: %v, but got: %v", expectedConfig, result)
	}

	// Test error case
	(*data)[parallelFilesUpload] = "invalid"
	_, err = GetBackupConfig(data)
	if !strings.Contains(err.Error(), "failed to parse ParallelFilesUpload") {
		t.Errorf("Expected error message containing 'failed to parse ParallelFilesUpload', but got: %v", err)
	}
}

func TestGetRestoreConfig(t *testing.T) {
	data := &map[string]string{
		writeSparseFiles: "true",
	}

	expectedConfig := velerov1api.UploaderConfigForRestore{
		WriteSparseFiles: true,
	}

	result, err := GetRestoreConfig(data)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !reflect.DeepEqual(result, expectedConfig) {
		t.Errorf("Expected: %v, but got: %v", expectedConfig, result)
	}

	// Test error case
	(*data)[writeSparseFiles] = "invalid"
	_, err = GetRestoreConfig(data)
	if !strings.Contains(err.Error(), "failed to parse WriteSparseFiles") {
		t.Errorf("Expected error message containing 'failed to parse WriteSparseFiles', but got: %v", err)
	}
}
