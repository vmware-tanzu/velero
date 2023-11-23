package uploaderconfig

import (
	"encoding/json"

	"github.com/pkg/errors"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

const (
	PodVolumeBackups  = "PodVolumeBackups"
	PodVolumeRestores = "PodVolumeRestores"
	DataUploads       = "DataUploads"
	DataDownloads     = "DataDownloads"
)

type PVBConfig struct {
	ParallelFilesUpload int `json:"parallelFilesUpload,omitempty"`
}

type PVRConfig struct {
	WriteSparseFiles bool `json:"writeSparseFiles,omitempty"`
}

func MarshalToPVBConfig(backupConfig *velerov1api.BackupConfig) (string, error) {
	jsonData, err := json.Marshal(backupConfig)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal backup config")
	}

	var pvb PVBConfig
	err = json.Unmarshal(jsonData, &pvb)
	if err != nil {
		return "", errors.Wrap(err, "failed to unmarshal backup config")
	}

	finalJSONData, err := json.Marshal(pvb)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal backup config")
	}

	return string(finalJSONData), nil
}

func MarshalToPVRConfig(restoreConfig *velerov1api.RestoreConfig) (string, error) {
	jsonData, err := json.Marshal(restoreConfig)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal restore config")
	}

	var pvr PVRConfig
	err = json.Unmarshal(jsonData, &pvr)
	if err != nil {
		return "", errors.Wrap(err, "failed to unmarshal restore config")
	}

	finalJSONData, err := json.Marshal(pvr)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal restore config")
	}

	return string(finalJSONData), nil
}

func ParseBackupConfig(str string) (velerov1api.BackupConfig, error) {
	var config velerov1api.BackupConfig
	err := json.Unmarshal([]byte(str), &config)
	if err != nil {
		return velerov1api.BackupConfig{}, err
	}
	return config, nil
}

func ParseRestoreConfig(str string) (velerov1api.RestoreConfig, error) {
	var config velerov1api.RestoreConfig
	err := json.Unmarshal([]byte(str), &config)
	if err != nil {
		return velerov1api.RestoreConfig{}, err
	}
	return config, nil
}
