package uploaderconfig

import (
	"strconv"

	"github.com/pkg/errors"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

const (
	parallelFilesUpload = "ParallelFilesUpload"
	writeSparseFiles    = "WriteSparseFiles"
)

func StoreBackupConfig(config *velerov1api.UploaderConfigForBackup) *map[string]string {
	data := make(map[string]string)
	data[parallelFilesUpload] = strconv.Itoa(config.ParallelFilesUpload)
	return &data
}

func StoreRestoreConfig(config *velerov1api.UploaderConfigForRestore) *map[string]string {
	data := make(map[string]string)
	data[writeSparseFiles] = strconv.FormatBool(config.WriteSparseFiles)
	return &data
}

func GetBackupConfig(data *map[string]string) (velerov1api.UploaderConfigForBackup, error) {
	config := velerov1api.UploaderConfigForBackup{}
	var err error
	if item, ok := (*data)[parallelFilesUpload]; ok {
		config.ParallelFilesUpload, err = strconv.Atoi(item)
		if err != nil {
			return velerov1api.UploaderConfigForBackup{}, errors.Wrap(err, "failed to parse ParallelFilesUpload")
		}
	}
	return config, nil
}

func GetRestoreConfig(data *map[string]string) (velerov1api.UploaderConfigForRestore, error) {
	config := velerov1api.UploaderConfigForRestore{}
	var err error
	if item, ok := (*data)[writeSparseFiles]; ok {
		config.WriteSparseFiles, err = strconv.ParseBool(item)
		if err != nil {
			return velerov1api.UploaderConfigForRestore{}, errors.Wrap(err, "failed to parse WriteSparseFiles")
		}
	}
	return config, nil
}
