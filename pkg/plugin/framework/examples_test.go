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

package framework

import (
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

func ExampleNewServer_blockStore() {
	NewServer(). // call the server
			RegisterBlockStore("example-blockstore", newBlockStore). // register the plugin
			Serve()                                                  // serve the plugin
}

func newBlockStore(logger logrus.FieldLogger) (interface{}, error) {
	return &BlockStore{FieldLogger: logger}, nil
}

type BlockStore struct {
	FieldLogger logrus.FieldLogger
}

// Implement all methods for the BlockStore interface...
func (b *BlockStore) Init(config map[string]string) error {
	b.FieldLogger.Infof("BlockStore.Init called")

	// ...

	return nil
}

func (b *BlockStore) CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (volumeID string, err error) {
	b.FieldLogger.Infof("CreateVolumeFromSnapshot called")

	// ...

	return "volumeID", nil
}

func (b *BlockStore) GetVolumeID(pv runtime.Unstructured) (string, error) {
	b.FieldLogger.Infof("GetVolumeID called")

	// ...

	return "volumeID", nil
}

func (b *BlockStore) SetVolumeID(pv runtime.Unstructured, volumeID string) (runtime.Unstructured, error) {
	b.FieldLogger.Infof("SetVolumeID called")

	// ...

	return nil, nil
}

func (b *BlockStore) GetVolumeInfo(volumeID, volumeAZ string) (string, *int64, error) {
	b.FieldLogger.Infof("GetVolumeInfo called")

	// ...

	return "volumeFilesystemType", nil, nil
}

func (b *BlockStore) CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (snapshotID string, err error) {
	b.FieldLogger.Infof("CreateSnapshot called")

	// ...

	return "snapshotID", nil
}

func (b *BlockStore) DeleteSnapshot(snapshotID string) error {
	b.FieldLogger.Infof("DeleteSnapshot called")

	// ...

	return nil
}
