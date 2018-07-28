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
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

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
	log logrus.FieldLogger
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

func NewBlockStore(logger logrus.FieldLogger) cloudprovider.BlockStore {
	return &blockStore{log: logger}
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
	// describe the snapshot so we can apply its tags to the volume
	snapReq := &ec2.DescribeSnapshotsInput{
		SnapshotIds: []*string{&snapshotID},
	}

	snapRes, err := b.ec2.DescribeSnapshots(snapReq)
	if err != nil {
		return "", errors.WithStack(err)
	}

	if count := len(snapRes.Snapshots); count != 1 {
		return "", errors.Errorf("expected 1 snapshot from DescribeSnapshots for %s, got %v", snapshotID, count)
	}

	req := &ec2.CreateVolumeInput{
		SnapshotId:       &snapshotID,
		AvailabilityZone: &volumeAZ,
		VolumeType:       &volumeType,
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String(ec2.ResourceTypeVolume),
				Tags:         snapRes.Snapshots[0].Tags,
			},
		},
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
	volumeInfo, err := b.describeVolume(volumeID)
	if err != nil {
		return "", nil, err
	}

	var (
		volumeType string
		iops       *int64
	)

	if volumeInfo.VolumeType != nil {
		volumeType = *volumeInfo.VolumeType
	}

	if iopsVolumeTypes.Has(volumeType) && volumeInfo.Iops != nil {
		iops = volumeInfo.Iops
	}

	return volumeType, iops, nil
}

func (b *blockStore) describeVolume(volumeID string) (*ec2.Volume, error) {
	req := &ec2.DescribeVolumesInput{
		VolumeIds: []*string{&volumeID},
	}

	res, err := b.ec2.DescribeVolumes(req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if count := len(res.Volumes); count != 1 {
		return nil, errors.Errorf("Expected one volume from DescribeVolumes for volume ID %v, got %v", volumeID, count)
	}

	return res.Volumes[0], nil
}

func (b *blockStore) CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (string, error) {
	// describe the volume so we can copy its tags to the snapshot
	volumeInfo, err := b.describeVolume(volumeID)
	if err != nil {
		return "", err
	}

	res, err := b.ec2.CreateSnapshot(&ec2.CreateSnapshotInput{
		VolumeId: &volumeID,
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String(ec2.ResourceTypeSnapshot),
				Tags:         getTags(tags, volumeInfo.Tags),
			},
		},
	})
	if err != nil {
		return "", errors.WithStack(err)
	}

	return *res.SnapshotId, nil
}

func getTags(arkTags map[string]string, volumeTags []*ec2.Tag) []*ec2.Tag {
	var result []*ec2.Tag

	// set Ark-assigned tags
	for k, v := range arkTags {
		result = append(result, ec2Tag(k, v))
	}

	// copy tags from volume to snapshot
	for _, tag := range volumeTags {
		// we want current Ark-assigned tags to overwrite any older versions
		// of them that may exist due to prior snapshots/restores
		if _, found := arkTags[*tag.Key]; found {
			continue
		}

		result = append(result, ec2Tag(*tag.Key, *tag.Value))
	}

	return result
}

func ec2Tag(key, val string) *ec2.Tag {
	return &ec2.Tag{Key: &key, Value: &val}
}

func (b *blockStore) DeleteSnapshot(snapshotID string) error {
	req := &ec2.DeleteSnapshotInput{
		SnapshotId: &snapshotID,
	}

	_, err := b.ec2.DeleteSnapshot(req)

	// if it's a NotFound error, we don't need to return an error
	// since the snapshot is not there.
	// see https://docs.aws.amazon.com/AWSEC2/latest/APIReference/errors-overview.html
	if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == "InvalidSnapshot.NotFound" {
		return nil
	}
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
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
