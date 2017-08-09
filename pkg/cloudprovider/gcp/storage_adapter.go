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

package gcp

import (
	"fmt"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v0.beta"
	"google.golang.org/api/storage/v1"

	"github.com/heptio/ark/pkg/cloudprovider"
)

type storageAdapter struct {
	blockStorage  *blockStorageAdapter
	objectStorage *objectStorageAdapter
}

var _ cloudprovider.StorageAdapter = &storageAdapter{}

func NewStorageAdapter(project string, zone string) (cloudprovider.StorageAdapter, error) {
	client, err := google.DefaultClient(oauth2.NoContext, compute.ComputeScope, storage.DevstorageReadWriteScope)

	if err != nil {
		return nil, err
	}

	gce, err := compute.New(client)
	if err != nil {
		return nil, err
	}

	// validate project & zone
	res, err := gce.Zones.Get(project, zone).Do()
	if err != nil {
		return nil, err
	}

	if res == nil {
		return nil, fmt.Errorf("zone %q not found for project %q", project, zone)
	}

	gcs, err := storage.New(client)
	if err != nil {
		return nil, err
	}

	return &storageAdapter{
		objectStorage: &objectStorageAdapter{
			gcs: gcs,
		},
		blockStorage: &blockStorageAdapter{
			gce:     gce,
			project: project,
			zone:    zone,
		},
	}, nil
}

func (op *storageAdapter) ObjectStorage() cloudprovider.ObjectStorageAdapter {
	return op.objectStorage
}

func (op *storageAdapter) BlockStorage() cloudprovider.BlockStorageAdapter {
	return op.blockStorage
}
