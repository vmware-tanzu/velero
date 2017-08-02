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

package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/heptio/ark/pkg/cloudprovider"
)

type storageAdapter struct {
	blockStorage  *blockStorageAdapter
	objectStorage *objectStorageAdapter
}

var _ cloudprovider.StorageAdapter = &storageAdapter{}

func NewStorageAdapter(config *aws.Config, availabilityZone string) (cloudprovider.StorageAdapter, error) {
	sess, err := session.NewSession(config)
	if err != nil {
		return nil, err
	}

	if _, err := sess.Config.Credentials.Get(); err != nil {
		return nil, err
	}

	return &storageAdapter{
		blockStorage: &blockStorageAdapter{
			ec2: ec2.New(sess),
			az:  availabilityZone,
		},
		objectStorage: &objectStorageAdapter{
			s3: s3.New(sess),
		},
	}, nil
}

func (op *storageAdapter) ObjectStorage() cloudprovider.ObjectStorageAdapter {
	return op.objectStorage
}

func (op *storageAdapter) BlockStorage() cloudprovider.BlockStorageAdapter {
	return op.blockStorage
}
