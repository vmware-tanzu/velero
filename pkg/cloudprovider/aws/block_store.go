/*
Copyright 2017 the Heptio Ark contributors.

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
	"regexp"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/util/collections"
)

const regionKey = "region"

// iopsVolumeTypes is a set of AWS EBS volume types for which IOPS should
// be captured during snapshot and provided when creating a new volume
// from snapshot.
var iopsVolumeTypes = sets.NewString("io1")

type blockStore struct {
	ec2 *ec2.EC2
}

func getSession(config *aws.Config) (*session.Session, error) {
	sess, err := session.NewSession(config)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if _, err := sess.Config.Credentials.Get(); err != nil {
		return nil, errors.WithStack(err)
	}

	return sess, nil
}

func NewBlockStore() cloudprovider.BlockStore {
	return &blockStore{}
}

func (b *blockStore) Init(config map[string]string) error {
	region := config[regionKey]
	if region == "" {
		return errors.Errorf("missing %s in aws configuration", regionKey)
	}

	awsConfig := aws.NewConfig().WithRegion(region)

	sess, err := getSession(awsConfig)
	if err != nil {
		return err
	}

	b.ec2 = ec2.New(sess)

	return nil
}

func (b *blockStore) CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (volumeID string, err error) {
	req := &ec2.CreateVolumeInput{
		SnapshotId:       &snapshotID,
		AvailabilityZone: &volumeAZ,
		VolumeType:       &volumeType,
	}

	if iopsVolumeTypes.Has(volumeType) && iops != nil {
		req.Iops = iops
	}

	res, err := b.ec2.CreateVolume(req)
	if err != nil {
		return "", errors.WithStack(err)
	}

	return *res.VolumeId, nil
}

func (b *blockStore) GetVolumeInfo(volumeID, volumeAZ string) (string, *int64, error) {
	req := &ec2.DescribeVolumesInput{
		VolumeIds: []*string{&volumeID},
	}

	res, err := b.ec2.DescribeVolumes(req)
	if err != nil {
		return "", nil, errors.WithStack(err)
	}

	if len(res.Volumes) != 1 {
		return "", nil, errors.Errorf("Expected one volume from DescribeVolumes for volume ID %v, got %v", volumeID, len(res.Volumes))
	}

	vol := res.Volumes[0]

	var (
		volumeType string
		iops       *int64
	)

	if vol.VolumeType != nil {
		volumeType = *vol.VolumeType
	}

	if iopsVolumeTypes.Has(volumeType) && vol.Iops != nil {
		iops = vol.Iops
	}

	return volumeType, iops, nil
}

func (b *blockStore) IsVolumeReady(volumeID, volumeAZ string) (ready bool, err error) {
	req := &ec2.DescribeVolumesInput{
		VolumeIds: []*string{&volumeID},
	}

	res, err := b.ec2.DescribeVolumes(req)
	if err != nil {
		return false, errors.WithStack(err)
	}
	if len(res.Volumes) != 1 {
		return false, errors.Errorf("Expected one volume from DescribeVolumes for volume ID %v, got %v", volumeID, len(res.Volumes))
	}

	return *res.Volumes[0].State == ec2.VolumeStateAvailable, nil
}

func (b *blockStore) CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (string, error) {
	req := &ec2.CreateSnapshotInput{
		VolumeId: &volumeID,
	}

	res, err := b.ec2.CreateSnapshot(req)
	if err != nil {
		return "", errors.WithStack(err)
	}

	tagsReq := &ec2.CreateTagsInput{}
	tagsReq.SetResources([]*string{res.SnapshotId})

	ec2Tags := make([]*ec2.Tag, 0, len(tags))

	for k, v := range tags {
		key := k
		val := v

		tag := &ec2.Tag{Key: &key, Value: &val}
		ec2Tags = append(ec2Tags, tag)
	}

	tagsReq.SetTags(ec2Tags)

	_, err = b.ec2.CreateTags(tagsReq)

	return *res.SnapshotId, errors.WithStack(err)
}

func (b *blockStore) DeleteSnapshot(snapshotID string) error {
	req := &ec2.DeleteSnapshotInput{
		SnapshotId: &snapshotID,
	}

	_, err := b.ec2.DeleteSnapshot(req)

	return errors.WithStack(err)
}

var ebsVolumeIDRegex = regexp.MustCompile("vol-.*")

func (b *blockStore) GetVolumeID(pv runtime.Unstructured) (string, error) {
	if !collections.Exists(pv.UnstructuredContent(), "spec.awsElasticBlockStore") {
		return "", nil
	}

	volumeID, err := collections.GetString(pv.UnstructuredContent(), "spec.awsElasticBlockStore.volumeID")
	if err != nil {
		return "", err
	}

	return ebsVolumeIDRegex.FindString(volumeID), nil
}

func (b *blockStore) SetVolumeID(pv runtime.Unstructured, volumeID string) (runtime.Unstructured, error) {
	aws, err := collections.GetMap(pv.UnstructuredContent(), "spec.awsElasticBlockStore")
	if err != nil {
		return nil, err
	}

	aws["volumeID"] = volumeID

	return pv, nil
}
