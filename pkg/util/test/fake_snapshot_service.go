/*
Copyright 2017 Heptio Inc.

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

package test

import (
	"errors"

	"k8s.io/apimachinery/pkg/util/sets"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
)

type FakeSnapshotService struct {
	// SnapshotID->VolumeID
	SnapshotsTaken sets.String

	// VolumeID -> (SnapshotID, Type, Iops)
	SnapshottableVolumes map[string]api.VolumeBackupInfo

	// VolumeBackupInfo -> VolumeID
	RestorableVolumes map[api.VolumeBackupInfo]string
}

func (s *FakeSnapshotService) GetAllSnapshots() ([]string, error) {
	return s.SnapshotsTaken.List(), nil
}

func (s *FakeSnapshotService) CreateSnapshot(volumeID, volumeAZ string) (string, error) {
	if _, exists := s.SnapshottableVolumes[volumeID]; !exists {
		return "", errors.New("snapshottable volume not found")
	}

	if s.SnapshotsTaken == nil {
		s.SnapshotsTaken = sets.NewString()
	}
	s.SnapshotsTaken.Insert(s.SnapshottableVolumes[volumeID].SnapshotID)

	return s.SnapshottableVolumes[volumeID].SnapshotID, nil
}

func (s *FakeSnapshotService) CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (string, error) {
	key := api.VolumeBackupInfo{
		SnapshotID:       snapshotID,
		Type:             volumeType,
		Iops:             iops,
		AvailabilityZone: volumeAZ,
	}

	return s.RestorableVolumes[key], nil
}

func (s *FakeSnapshotService) DeleteSnapshot(snapshotID string) error {
	if !s.SnapshotsTaken.Has(snapshotID) {
		return errors.New("snapshot not found")
	}

	s.SnapshotsTaken.Delete(snapshotID)

	return nil
}

func (s *FakeSnapshotService) GetVolumeInfo(volumeID, volumeAZ string) (string, *int64, error) {
	if volumeInfo, exists := s.SnapshottableVolumes[volumeID]; !exists {
		return "", nil, errors.New("VolumeID not found")
	} else {
		return volumeInfo.Type, volumeInfo.Iops, nil
	}
}
