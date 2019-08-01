/*
Copyright 2017, 2019 the Velero contributors.

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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
)

type VolumeBackupInfo struct {
	SnapshotID       string
	Type             string
	Iops             *int64
	AvailabilityZone string
}

type FakeVolumeSnapshotter struct {
	// SnapshotID->VolumeID
	SnapshotsTaken sets.String

	// VolumeID -> (SnapshotID, Type, Iops)
	SnapshottableVolumes map[string]VolumeBackupInfo

	// VolumeBackupInfo -> VolumeID
	RestorableVolumes map[VolumeBackupInfo]string

	VolumeID    string
	VolumeIDSet string

	Error error
}

func (bs *FakeVolumeSnapshotter) Init(config map[string]string) error {
	return nil
}

func (bs *FakeVolumeSnapshotter) CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (string, error) {
	if bs.Error != nil {
		return "", bs.Error
	}

	if _, exists := bs.SnapshottableVolumes[volumeID]; !exists {
		return "", errors.New("snapshottable volume not found")
	}

	if bs.SnapshotsTaken == nil {
		bs.SnapshotsTaken = sets.NewString()
	}
	bs.SnapshotsTaken.Insert(bs.SnapshottableVolumes[volumeID].SnapshotID)

	return bs.SnapshottableVolumes[volumeID].SnapshotID, nil
}

func (bs *FakeVolumeSnapshotter) CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (string, error) {
	if bs.Error != nil {
		return "", bs.Error
	}

	key := VolumeBackupInfo{
		SnapshotID:       snapshotID,
		Type:             volumeType,
		Iops:             iops,
		AvailabilityZone: volumeAZ,
	}

	return bs.RestorableVolumes[key], nil
}

func (bs *FakeVolumeSnapshotter) DeleteSnapshot(snapshotID string) error {
	if bs.Error != nil {
		return bs.Error
	}

	if !bs.SnapshotsTaken.Has(snapshotID) {
		return errors.New("snapshot not found")
	}

	bs.SnapshotsTaken.Delete(snapshotID)

	return nil
}

func (bs *FakeVolumeSnapshotter) GetVolumeInfo(volumeID, volumeAZ string) (string, *int64, error) {
	if bs.Error != nil {
		return "", nil, bs.Error
	}

	if volumeInfo, exists := bs.SnapshottableVolumes[volumeID]; !exists {
		return "", nil, errors.New("VolumeID not found")
	} else {
		return volumeInfo.Type, volumeInfo.Iops, nil
	}
}

func (bs *FakeVolumeSnapshotter) GetVolumeID(pv runtime.Unstructured) (string, error) {
	return bs.VolumeID, nil
}

func (bs *FakeVolumeSnapshotter) SetVolumeID(pv runtime.Unstructured, volumeID string) (runtime.Unstructured, error) {
	bs.VolumeIDSet = volumeID
	return pv, bs.Error
}
