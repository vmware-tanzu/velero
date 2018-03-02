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
	"os"

	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/util/collections"
)

const projectKey = "project"

type blockStore struct {
	gce     *compute.Service
	project string
}

func NewBlockStore() cloudprovider.BlockStore {
	return &blockStore{}
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

	// validate connection
	res, err := gce.Projects.Get(project).Do()
	if err != nil {
		return errors.WithStack(err)
	}

	if res == nil {
		return errors.Errorf("error getting project %q", project)
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

func (b *blockStore) CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (volumeID string, err error) {
	res, err := b.gce.Snapshots.Get(b.project, snapshotID).Do()
	if err != nil {
		return "", errors.WithStack(err)
	}

	disk := &compute.Disk{
		Name:           "restore-" + uuid.NewV4().String(),
		SourceSnapshot: res.SelfLink,
		Type:           volumeType,
	}

	if _, err = b.gce.Disks.Insert(b.project, volumeAZ, disk).Do(); err != nil {
		return "", errors.WithStack(err)
	}

	return disk.Name, nil
}

func (b *blockStore) GetVolumeInfo(volumeID, volumeAZ string) (string, *int64, error) {
	res, err := b.gce.Disks.Get(b.project, volumeAZ, volumeID).Do()
	if err != nil {
		return "", nil, errors.WithStack(err)
	}

	return res.Type, nil, nil
}

func (b *blockStore) IsVolumeReady(volumeID, volumeAZ string) (ready bool, err error) {
	disk, err := b.gce.Disks.Get(b.project, volumeAZ, volumeID).Do()
	if err != nil {
		return false, errors.WithStack(err)
	}

	// TODO can we consider a disk ready while it's in the RESTORING state?
	return disk.Status == "READY", nil
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

	gceSnap := compute.Snapshot{
		Name:   snapshotName,
		Labels: tags,
	}

	_, err := b.gce.Disks.CreateSnapshot(b.project, volumeAZ, volumeID, &gceSnap).Do()
	if err != nil {
		return "", errors.WithStack(err)
	}

	return gceSnap.Name, nil
}

func (b *blockStore) DeleteSnapshot(snapshotID string) error {
	_, err := b.gce.Snapshots.Delete(b.project, snapshotID).Do()

	return errors.WithStack(err)
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
