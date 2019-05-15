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

package alibabacloud

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/heptio/velero/pkg/cloudprovider"
)

const ackClusterNameKey = "ACK_CLUSTER_NAME"

type VolumeSnapshotter struct {
	log logrus.FieldLogger
	ecs *ecs.Client
}

func NewVolumeSnapshotter(logger logrus.FieldLogger) *VolumeSnapshotter {
	return &VolumeSnapshotter{log: logger}
}

func (b *VolumeSnapshotter) Init(config map[string]string) error {
	if err := cloudprovider.ValidateVolumeSnapshotterConfigKeys(config, regionKey); err != nil {
		return err
	}

	if err := loadEnv(); err != nil {
		return err
	}

	region := config[regionKey]
	if region == "" {
		return errors.Errorf("missing %s in Alibaba Cloud configuration", regionKey)
	}

	accessKeyId := os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_ID")
	accessKeySecret := os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET")
	stsToken := os.Getenv("ALIBABA_CLOUD_ACCESS_STS_TOKEN")

	if len(accessKeyId) == 0 {
		return errors.Errorf("ALIBABA_CLOUD_ACCESS_KEY_ID environment variable is not set")
	}
	if len(accessKeySecret) == 0 {
		return errors.Errorf("ALIBABA_CLOUD_ACCESS_KEY_SECRET environment variable is not set")
	}

	var client *ecs.Client
	var err error

	if len(stsToken) == 0 {
		client, err = ecs.NewClientWithAccessKey(region, accessKeyId, accessKeySecret)
	} else {
		client, err = ecs.NewClientWithStsToken(region, accessKeyId, accessKeySecret, stsToken)
	}
	b.ecs = client
	return err
}

func getJSONArrayString(id string) string {
	return fmt.Sprintf("[\"%s\"]", id)
}

func (b *VolumeSnapshotter) CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (volumeID string, err error) {
	// describe the snapshot so we can apply its tags to the volume
	snapReq := ecs.CreateDescribeSnapshotsRequest()
	snapReq.SnapshotIds = getJSONArrayString(snapshotID)

	snapRes, err := b.ecs.DescribeSnapshots(snapReq)
	if err != nil {
		return "", errors.WithStack(err)
	}

	if count := len(snapRes.Snapshots.Snapshot); count != 1 {
		return "", errors.Errorf("expected 1 snapshot from DescribeSnapshots for %s, got %v", snapshotID, count)
	}

	tags := getTagsForCluster(snapRes.Snapshots.Snapshot[0].Tags.Tag)

	// filter tags through getTagsForCluster() function in order to apply
	// proper ownership tags to restored volumes
	req := ecs.CreateCreateDiskRequest()
	req.SnapshotId = snapshotID
	req.ZoneId = volumeAZ
	req.DiskCategory = volumeType
	req.Encrypted = requests.NewBoolean(snapRes.Snapshots.Snapshot[0].Encrypted)
	req.Tag = &tags

	res, err := b.ecs.CreateDisk(req)
	if err != nil {
		return "", errors.WithStack(err)
	}

	return res.DiskId, nil
}

func (b *VolumeSnapshotter) GetVolumeInfo(volumeID, volumeAZ string) (string, *int64, error) {

	volumeInfo, err := b.describeVolume(volumeID, volumeAZ)

	if err != nil {
		return "", nil, err
	}

	iops := int64(volumeInfo.IOPS)

	return volumeInfo.Category, &iops, nil
}

func (b *VolumeSnapshotter) describeVolume(volumeID string, volumeAZ string) (*ecs.Disk, error) {
	req := ecs.CreateDescribeDisksRequest()

	req.DiskIds = getJSONArrayString(volumeID)
	req.ZoneId = volumeAZ

	res, err := b.ecs.DescribeDisks(req)

	if err != nil {
		return nil, errors.WithStack(err)
	}

	if count := len(res.Disks.Disk); count != 1 {
		return nil, errors.Errorf("Expected one volume from DescribeDisks for volume ID %v, got %v", volumeID, count)
	}

	return &res.Disks.Disk[0], nil
}

func (b *VolumeSnapshotter) CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (string, error) {
	// describe the volume so we can copy its tags to the snapshot
	volumeInfo, err := b.describeVolume(volumeID, volumeAZ)
	if err != nil {
		return "", err
	}
	req := ecs.CreateCreateSnapshotRequest()
	req.DiskId = volumeID

	newTags := getTags(tags, volumeInfo.Tags.Tag)

	if len(tags) > 0 {
		req.Tag = &newTags
	}

	res, err := b.ecs.CreateSnapshot(req)
	if err != nil {
		return "", errors.WithStack(err)
	}

	return res.SnapshotId, nil
}

func getTagsForCluster(snapshotTags []ecs.Tag) []ecs.CreateDiskTag {
	var result []ecs.CreateDiskTag

	clusterName, haveACKClusterNameEnvVar := os.LookupEnv(ackClusterNameKey)

	if haveACKClusterNameEnvVar {
		result = append(result, ecs.CreateDiskTag{
			Key:   "kubernetes.io/cluster/" + clusterName,
			Value: "owned",
		})

		result = append(result, ecs.CreateDiskTag{
			Key:   "KubernetesCluster",
			Value: clusterName,
		})
	}

	for _, tag := range snapshotTags {
		if haveACKClusterNameEnvVar && (strings.HasPrefix(tag.TagKey, "kubernetes.io/cluster/") || tag.TagKey == "KubernetesCluster") {
			// if the ACK_CLUSTER_NAME variable is found we want current cluster
			// to overwrite the old ownership on volumes
			continue
		}

		result = append(result, ecs.CreateDiskTag{
			Key:   tag.TagKey,
			Value: tag.TagValue,
		})
	}

	return result
}

func getTags(veleroTags map[string]string, volumeTags []ecs.Tag) []ecs.CreateSnapshotTag {
	var result []ecs.CreateSnapshotTag

	// set Velero-assigned tags
	for k, v := range veleroTags {
		result = append(result, ecs.CreateSnapshotTag{
			Key:   k,
			Value: v,
		})
	}

	// copy tags from volume to snapshot
	for _, tag := range volumeTags {
		// we want current Velero-assigned tags to overwrite any older versions
		// of them that may exist due to prior snapshots/restores
		if _, found := veleroTags[tag.TagKey]; found {
			continue
		}

		result = append(result, ecs.CreateSnapshotTag{
			Key:   tag.TagKey,
			Value: tag.TagValue,
		})
	}

	return result
}

func (b *VolumeSnapshotter) DeleteSnapshot(snapshotID string) error {
	req := ecs.CreateDeleteSnapshotRequest()
	req.SnapshotId = snapshotID

	_, err := b.ecs.DeleteSnapshot(req)

	// if it's a NotFound error, the request will be ignored

	if err != nil {
		return errors.WithStack(err)
	}

	return err
}

var ebsVolumeIDRegex = regexp.MustCompile("vol-.*")

func checkCSIVolumeDriver(driver string) error {
	if driver != "diskplugin.csi.alibabacloud.com" {
		return errors.New("unsupported CSI driver: " + driver)
	}
	return nil
}

func checkFlexVolumeDriver(driver string) error {
	if driver != "alicloud/disk" {
		return errors.New("unsupported FlexVolume driver: " + driver)
	}
	return nil
}

func getEBSDiskID(pv *v1.PersistentVolume) (string, error) {
	if pv.Spec.CSI != nil {
		err := checkCSIVolumeDriver(pv.Spec.CSI.Driver)
		if err != nil {
			return "", err
		}
		handle := pv.Spec.CSI.VolumeHandle
		if handle == "" {
			return "", errors.New("spec.CSI.VolumeHandle not found")
		}
		return handle, nil
	} else if pv.Spec.FlexVolume != nil {
		err := checkFlexVolumeDriver(pv.Spec.FlexVolume.Driver)
		if err != nil {
			return "", err
		}
		options := pv.Spec.FlexVolume.Options
		if options == nil || options["volumeId"] == "" {
			return "", errors.New("spec.FlexVolume.Options['volumeId'] not found")
		}
		return options["volumeId"], nil
	}
	return "", nil
}

func setEBSDiskID(pv *v1.PersistentVolume, diskID string) error {
	if pv.Spec.CSI != nil {
		err := checkCSIVolumeDriver(pv.Spec.CSI.Driver)
		if err != nil {
			return err
		}
		pv.Spec.CSI.VolumeHandle = diskID
		return nil

	} else if pv.Spec.FlexVolume != nil {
		err := checkFlexVolumeDriver(pv.Spec.FlexVolume.Driver)
		if err != nil {
			return err
		}
		options := pv.Spec.FlexVolume.Options
		if options == nil {
			options = map[string]string{}
			pv.Spec.FlexVolume.Options = options
		}
		options["volumeId"] = diskID
		return nil
	}

	return errors.New("spec.CSI or spec.FlexVolume not found")
}

func (b *VolumeSnapshotter) GetVolumeID(unstructuredPV runtime.Unstructured) (string, error) {
	pv := new(v1.PersistentVolume)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredPV.UnstructuredContent(), pv); err != nil {
		return "", errors.WithStack(err)
	}

	volumeID, err := getEBSDiskID(pv)

	if err != nil {
		return "", err
	}

	return volumeID, nil
}

func (b *VolumeSnapshotter) SetVolumeID(unstructuredPV runtime.Unstructured, volumeID string) (runtime.Unstructured, error) {
	pv := new(v1.PersistentVolume)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredPV.UnstructuredContent(), pv); err != nil {
		return nil, errors.WithStack(err)
	}

	//pvFailureDomainZone := pv.Labels["failure-domain.beta.kubernetes.io/zone"]

	err := setEBSDiskID(pv, volumeID)

	if err != nil {
		return nil, err
	}

	res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pv)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &unstructured.Unstructured{Object: res}, nil
}
