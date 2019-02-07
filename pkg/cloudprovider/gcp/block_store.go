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

package gcp

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/satori/uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/heptio/velero/pkg/cloudprovider"
	"github.com/heptio/velero/pkg/util/collections"
)

const (
	projectKey    = "project"
	zoneSeparator = "__"
)

type blockStore struct {
	gce     *compute.Service
	project string
	log     logrus.FieldLogger
}

func NewBlockStore(logger logrus.FieldLogger) cloudprovider.BlockStore {
	return &blockStore{log: logger}
}

func (b *blockStore) Init(config map[string]string) error {
	project, err := extractProjectFromCreds()
	if err != nil {
		return err
	}

	client, err := google.DefaultClient(oauth2.NoContext, compute.ComputeScope)
	if err != nil {
		return errors.WithStack(err)
	}

	gce, err := compute.New(client)
	if err != nil {
		return errors.WithStack(err)
	}

	b.gce = gce
	b.project = project

	return nil
}

func extractProjectFromCreds() (string, error) {
	credsBytes, err := ioutil.ReadFile(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	if err != nil {
		return "", errors.WithStack(err)
	}

	type credentials struct {
		ProjectID string `json:"project_id"`
	}

	var creds credentials
	if err := json.Unmarshal(credsBytes, &creds); err != nil {
		return "", errors.WithStack(err)
	}

	if creds.ProjectID == "" {
		return "", errors.New("cannot fetch project_id from GCP credentials file")
	}

	return creds.ProjectID, nil
}

// isMultiZone returns true if the failure-domain tag contains
// double underscore, which is the separator used
// by GKE when a storage class spans multiple availablity
// zones.
func isMultiZone(volumeAZ string) bool {
	return strings.Contains(volumeAZ, zoneSeparator)
}

// parseRegion parses a failure-domain tag with multiple zones
// and returns a single region. Zones are sperated by double underscores (__).
// For example
//     input: us-central1-a__us-central1-b
//     return: us-central1
// When a custom storage class spans multiple geographical zones,
// such as us-central1 and us-west1 only the zone matching the cluster is used
// in the failure-domain tag.
// For example
//     Cluster nodes in us-central1-c, us-central1-f
//     Storage class zones us-central1-a, us-central1-f, us-east1-a, us-east1-d
//     The failure-domain tag would be: us-central1-a__us-central1-f
func parseRegion(volumeAZ string) (string, error) {
	zones := strings.Split(volumeAZ, zoneSeparator)
	zone := zones[0]
	parts := strings.SplitAfterN(zone, "-", 3)
	if len(parts) < 2 {
		return "", errors.Errorf("failed to parse region from zone: %q", volumeAZ)
	}
	return parts[0] + strings.TrimSuffix(parts[1], "-"), nil
}

// Retrieve the URLs for zones via the GCP API.
func (b *blockStore) getZoneURLs(volumeAZ string) ([]string, error) {
	zones := strings.Split(volumeAZ, zoneSeparator)
	var zoneURLs []string
	for _, z := range zones {
		zone, err := b.gce.Zones.Get(b.project, z).Do()
		if err != nil {
			return nil, errors.WithStack(err)
		}

		zoneURLs = append(zoneURLs, zone.SelfLink)
	}

	return zoneURLs, nil
}

func (b *blockStore) CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (volumeID string, err error) {
	// get the snapshot so we can apply its tags to the volume
	res, err := b.gce.Snapshots.Get(b.project, snapshotID).Do()
	if err != nil {
		return "", errors.WithStack(err)
	}

	// Kubernetes uses the description field of GCP disks to store a JSON doc containing
	// tags.
	//
	// use the snapshot's description (which contains tags from the snapshotted disk
	// plus Velero-specific tags) to set the new disk's description.
	disk := &compute.Disk{
		Name:           "restore-" + uuid.NewV4().String(),
		SourceSnapshot: res.SelfLink,
		Type:           volumeType,
		Description:    res.Description,
	}

	if isMultiZone(volumeAZ) {
		volumeRegion, err := parseRegion(volumeAZ)
		if err != nil {
			return "", err
		}

		// URLs for zones that the volume is replicated to within GCP
		zoneURLs, err := b.getZoneURLs(volumeAZ)
		if err != nil {
			return "", err
		}

		disk.ReplicaZones = zoneURLs

		if _, err = b.gce.RegionDisks.Insert(b.project, volumeRegion, disk).Do(); err != nil {
			return "", errors.WithStack(err)
		}
	} else {
		if _, err = b.gce.Disks.Insert(b.project, volumeAZ, disk).Do(); err != nil {
			return "", errors.WithStack(err)
		}
	}

	return disk.Name, nil
}

func (b *blockStore) GetVolumeInfo(volumeID, volumeAZ string) (string, *int64, error) {
	var (
		res *compute.Disk
		err error
	)

	if isMultiZone(volumeAZ) {
		volumeRegion, err := parseRegion(volumeAZ)
		if err != nil {
			return "", nil, errors.WithStack(err)
		}
		res, err = b.gce.RegionDisks.Get(b.project, volumeRegion, volumeID).Do()
		if err != nil {
			return "", nil, errors.WithStack(err)
		}
	} else {
		res, err = b.gce.Disks.Get(b.project, volumeAZ, volumeID).Do()
		if err != nil {
			return "", nil, errors.WithStack(err)
		}
	}
	return res.Type, nil, nil
}

func (b *blockStore) CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (string, error) {
	// snapshot names must adhere to RFC1035 and be 1-63 characters
	// long
	var snapshotName string
	suffix := "-" + uuid.NewV4().String()

	if len(volumeID) <= (63 - len(suffix)) {
		snapshotName = volumeID + suffix
	} else {
		snapshotName = volumeID[0:63-len(suffix)] + suffix
	}

	if isMultiZone(volumeAZ) {
		volumeRegion, err := parseRegion(volumeAZ)
		if err != nil {
			return "", errors.WithStack(err)
		}
		return b.createRegionSnapshot(snapshotName, volumeID, volumeRegion, tags)
	} else {
		return b.createSnapshot(snapshotName, volumeID, volumeAZ, tags)
	}
}

func (b *blockStore) createSnapshot(snapshotName, volumeID, volumeAZ string, tags map[string]string) (string, error) {
	disk, err := b.gce.Disks.Get(b.project, volumeAZ, volumeID).Do()
	if err != nil {
		return "", errors.WithStack(err)
	}

	gceSnap := compute.Snapshot{
		Name:        snapshotName,
		Description: getSnapshotTags(tags, disk.Description, b.log),
	}

	_, err = b.gce.Disks.CreateSnapshot(b.project, volumeAZ, volumeID, &gceSnap).Do()
	if err != nil {
		return "", errors.WithStack(err)
	}

	return gceSnap.Name, nil
}

func (b *blockStore) createRegionSnapshot(snapshotName, volumeID, volumeRegion string, tags map[string]string) (string, error) {
	disk, err := b.gce.RegionDisks.Get(b.project, volumeRegion, volumeID).Do()
	if err != nil {
		return "", errors.WithStack(err)
	}

	gceSnap := compute.Snapshot{
		Name:        snapshotName,
		Description: getSnapshotTags(tags, disk.Description, b.log),
	}

	_, err = b.gce.RegionDisks.CreateSnapshot(b.project, volumeRegion, volumeID, &gceSnap).Do()
	if err != nil {
		return "", errors.WithStack(err)
	}

	return gceSnap.Name, nil
}

func getSnapshotTags(veleroTags map[string]string, diskDescription string, log logrus.FieldLogger) string {
	// Kubernetes uses the description field of GCP disks to store a JSON doc containing
	// tags.
	//
	// use the tags in the disk's description (if a valid JSON doc) plus the tags arg
	// to set the snapshot's description.
	var snapshotTags map[string]string
	if err := json.Unmarshal([]byte(diskDescription), &snapshotTags); err != nil {
		// error decoding the disk's description, so just use the Velero-assigned tags
		log.WithError(err).
			Error("unable to decode disk's description as JSON, so only applying Velero-assigned tags to snapshot")
		snapshotTags = veleroTags
	} else {
		// merge Velero-assigned tags with the disk's tags (note that we want current
		// Velero-assigned tags to overwrite any older versions of them that may exist
		// due to prior snapshots/restores)
		for k, v := range veleroTags {
			snapshotTags[k] = v
		}
	}

	if len(snapshotTags) == 0 {
		return ""
	}

	tagsJSON, err := json.Marshal(snapshotTags)
	if err != nil {
		log.WithError(err).Error("unable to encode snapshot's tags to JSON, so not tagging snapshot")
		return ""
	}

	return string(tagsJSON)
}

func (b *blockStore) DeleteSnapshot(snapshotID string) error {
	_, err := b.gce.Snapshots.Delete(b.project, snapshotID).Do()

	// if it's a 404 (not found) error, we don't need to return an error
	// since the snapshot is not there.
	if gcpErr, ok := err.(*googleapi.Error); ok && gcpErr.Code == http.StatusNotFound {
		return nil
	}
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (b *blockStore) GetVolumeID(pv runtime.Unstructured) (string, error) {
	if !collections.Exists(pv.UnstructuredContent(), "spec.gcePersistentDisk") {
		return "", nil
	}

	volumeID, err := collections.GetString(pv.UnstructuredContent(), "spec.gcePersistentDisk.pdName")
	if err != nil {
		return "", err
	}

	return volumeID, nil
}

func (b *blockStore) SetVolumeID(pv runtime.Unstructured, volumeID string) (runtime.Unstructured, error) {
	gce, err := collections.GetMap(pv.UnstructuredContent(), "spec.gcePersistentDisk")
	if err != nil {
		return nil, err
	}

	gce["pdName"] = volumeID

	return pv, nil
}
