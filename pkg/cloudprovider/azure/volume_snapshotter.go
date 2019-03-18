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

package azure

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	disk "github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2018-04-01/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/heptio/velero/pkg/cloudprovider"
)

const (
	resourceGroupEnvVar = "AZURE_RESOURCE_GROUP"

	apiTimeoutConfigKey = "apiTimeout"

	snapshotsResource = "snapshots"
	disksResource     = "disks"
)

type VolumeSnapshotter struct {
	log                logrus.FieldLogger
	disks              *disk.DisksClient
	snaps              *disk.SnapshotsClient
	subscription       string
	disksResourceGroup string
	snapsResourceGroup string
	apiTimeout         time.Duration
}

type snapshotIdentifier struct {
	subscription  string
	resourceGroup string
	name          string
}

func (si *snapshotIdentifier) String() string {
	return getComputeResourceName(si.subscription, si.resourceGroup, snapshotsResource, si.name)
}

func NewVolumeSnapshotter(logger logrus.FieldLogger) *VolumeSnapshotter {
	return &VolumeSnapshotter{log: logger}
}

func (b *VolumeSnapshotter) Init(config map[string]string) error {
	if err := cloudprovider.ValidateVolumeSnapshotterConfigKeys(config, resourceGroupConfigKey, apiTimeoutConfigKey); err != nil {
		return err
	}

	// load environment vars from $AZURE_CREDENTIALS_FILE, if it exists
	if err := loadEnv(); err != nil {
		return err
	}

	// 1. we need AZURE_TENANT_ID, AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, AZURE_SUBSCRIPTION_ID, AZURE_RESOURCE_GROUP
	envVars, err := getRequiredValues(os.Getenv, tenantIDEnvVar, clientIDEnvVar, clientSecretEnvVar, subscriptionIDEnvVar, resourceGroupEnvVar)
	if err != nil {
		return errors.Wrap(err, "unable to get all required environment variables")
	}

	// 2. if config["apiTimeout"] is empty, default to 2m; otherwise, parse it
	var apiTimeout time.Duration
	if val := config[apiTimeoutConfigKey]; val == "" {
		apiTimeout = 2 * time.Minute
	} else {
		apiTimeout, err = time.ParseDuration(val)
		if err != nil {
			return errors.Wrapf(err, "unable to parse value %q for config key %q (expected a duration string)", val, apiTimeoutConfigKey)
		}
	}

	// 3. get SPT
	spt, err := newServicePrincipalToken(envVars[tenantIDEnvVar], envVars[clientIDEnvVar], envVars[clientSecretEnvVar], azure.PublicCloud.ResourceManagerEndpoint)
	if err != nil {
		return errors.Wrap(err, "error getting service principal token")
	}

	// 4. set up clients
	disksClient := disk.NewDisksClient(envVars[subscriptionIDEnvVar])
	snapsClient := disk.NewSnapshotsClient(envVars[subscriptionIDEnvVar])

	disksClient.PollingDelay = 5 * time.Second
	snapsClient.PollingDelay = 5 * time.Second

	authorizer := autorest.NewBearerAuthorizer(spt)
	disksClient.Authorizer = authorizer
	snapsClient.Authorizer = authorizer

	b.disks = &disksClient
	b.snaps = &snapsClient
	b.subscription = envVars[subscriptionIDEnvVar]
	b.disksResourceGroup = envVars[resourceGroupEnvVar]
	b.snapsResourceGroup = config[resourceGroupConfigKey]

	// if no resource group was explicitly specified in 'config',
	// use the value from the env var (i.e. the same one as where
	// the cluster & disks are)
	if b.snapsResourceGroup == "" {
		b.snapsResourceGroup = envVars[resourceGroupEnvVar]
	}

	b.apiTimeout = apiTimeout

	return nil
}

func (b *VolumeSnapshotter) CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (string, error) {
	snapshotIdentifier, err := parseFullSnapshotName(snapshotID)
	if err != nil {
		return "", err
	}

	// Lookup snapshot info for its Location & Tags so we can apply them to the volume
	snapshotInfo, err := b.snaps.Get(context.TODO(), snapshotIdentifier.resourceGroup, snapshotIdentifier.name)
	if err != nil {
		return "", errors.WithStack(err)
	}

	diskName := "restore-" + uuid.NewV4().String()

	disk := disk.Disk{
		Name:     &diskName,
		Location: snapshotInfo.Location,
		DiskProperties: &disk.DiskProperties{
			CreationData: &disk.CreationData{
				CreateOption:     disk.Copy,
				SourceResourceID: stringPtr(snapshotIdentifier.String()),
			},
		},
		Sku: &disk.DiskSku{
			Name: disk.StorageAccountTypes(volumeType),
		},
		Tags: snapshotInfo.Tags,
	}

	// Restore the disk in the correct zone
	regionParts := strings.Split(volumeAZ, "-")
	if len(regionParts) >= 2 {
		disk.Zones = &[]string{regionParts[len(regionParts)-1]}
	}

	ctx, cancel := context.WithTimeout(context.Background(), b.apiTimeout)
	defer cancel()

	future, err := b.disks.CreateOrUpdate(ctx, b.disksResourceGroup, *disk.Name, disk)
	if err != nil {
		return "", errors.WithStack(err)
	}
	if err = future.WaitForCompletionRef(ctx, b.disks.Client); err != nil {
		return "", errors.WithStack(err)
	}
	if _, err = future.Result(*b.disks); err != nil {
		return "", errors.WithStack(err)
	}

	return diskName, nil
}

func (b *VolumeSnapshotter) GetVolumeInfo(volumeID, volumeAZ string) (string, *int64, error) {
	res, err := b.disks.Get(context.TODO(), b.disksResourceGroup, volumeID)
	if err != nil {
		return "", nil, errors.WithStack(err)
	}

	if res.Sku == nil {
		return "", nil, errors.New("disk has a nil SKU")
	}

	return string(res.Sku.Name), nil, nil
}

func (b *VolumeSnapshotter) CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (string, error) {
	// Lookup disk info for its Location
	diskInfo, err := b.disks.Get(context.TODO(), b.disksResourceGroup, volumeID)
	if err != nil {
		return "", errors.WithStack(err)
	}

	fullDiskName := getComputeResourceName(b.subscription, b.disksResourceGroup, disksResource, volumeID)
	// snapshot names must be <= 80 characters long
	var snapshotName string
	suffix := "-" + uuid.NewV4().String()

	if len(volumeID) <= (80 - len(suffix)) {
		snapshotName = volumeID + suffix
	} else {
		snapshotName = volumeID[0:80-len(suffix)] + suffix
	}

	snap := disk.Snapshot{
		Name: &snapshotName,
		DiskProperties: &disk.DiskProperties{
			CreationData: &disk.CreationData{
				CreateOption:     disk.Copy,
				SourceResourceID: &fullDiskName,
			},
		},
		Tags:     getSnapshotTags(tags, diskInfo.Tags),
		Location: diskInfo.Location,
	}

	ctx, cancel := context.WithTimeout(context.Background(), b.apiTimeout)
	defer cancel()

	future, err := b.snaps.CreateOrUpdate(ctx, b.snapsResourceGroup, *snap.Name, snap)
	if err != nil {
		return "", errors.WithStack(err)
	}
	if err = future.WaitForCompletionRef(ctx, b.snaps.Client); err != nil {
		return "", errors.WithStack(err)
	}
	if _, err = future.Result(*b.snaps); err != nil {
		return "", errors.WithStack(err)
	}

	return getComputeResourceName(b.subscription, b.snapsResourceGroup, snapshotsResource, snapshotName), nil
}

func getSnapshotTags(veleroTags map[string]string, diskTags map[string]*string) map[string]*string {
	if diskTags == nil && len(veleroTags) == 0 {
		return nil
	}

	snapshotTags := make(map[string]*string)

	// copy tags from disk to snapshot
	if diskTags != nil {
		for k, v := range diskTags {
			snapshotTags[k] = stringPtr(*v)
		}
	}

	// merge Velero-assigned tags with the disk's tags (note that we want current
	// Velero-assigned tags to overwrite any older versions of them that may exist
	// due to prior snapshots/restores)
	for k, v := range veleroTags {
		// Azure does not allow slashes in tag keys, so replace
		// with dash (inline with what Kubernetes does)
		key := strings.Replace(k, "/", "-", -1)
		snapshotTags[key] = stringPtr(v)
	}

	return snapshotTags
}

func stringPtr(s string) *string {
	return &s
}

func (b *VolumeSnapshotter) DeleteSnapshot(snapshotID string) error {
	snapshotInfo, err := parseFullSnapshotName(snapshotID)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), b.apiTimeout)
	defer cancel()

	// we don't want to return an error if the snapshot doesn't exist, and
	// the Delete(..) call does not return a clear error if that's the case,
	// so first try to get it and return early if we get a 404.
	_, err = b.snaps.Get(ctx, snapshotInfo.resourceGroup, snapshotInfo.name)
	if azureErr, ok := err.(autorest.DetailedError); ok && azureErr.StatusCode == http.StatusNotFound {
		b.log.WithField("snapshotID", snapshotID).Debug("Snapshot not found")
		return nil
	}

	future, err := b.snaps.Delete(ctx, snapshotInfo.resourceGroup, snapshotInfo.name)
	if err != nil {
		return errors.WithStack(err)
	}
	if err = future.WaitForCompletionRef(ctx, b.snaps.Client); err != nil {
		b.log.WithError(err).Errorf("Error waiting for completion ref")
		return errors.WithStack(err)
	}
	_, err = future.Result(*b.snaps)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func getComputeResourceName(subscription, resourceGroup, resource, name string) string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/%s/%s", subscription, resourceGroup, resource, name)
}

var snapshotURIRegexp = regexp.MustCompile(
	`^\/subscriptions\/(?P<subscription>.*)\/resourceGroups\/(?P<resourceGroup>.*)\/providers\/Microsoft.Compute\/snapshots\/(?P<snapshotName>.*)$`)

// parseFullSnapshotName takes a fully-qualified snapshot name and returns
// a snapshot identifier or an error if the snapshot name does not match the
// regexp.
func parseFullSnapshotName(name string) (*snapshotIdentifier, error) {
	submatches := snapshotURIRegexp.FindStringSubmatch(name)
	if len(submatches) != len(snapshotURIRegexp.SubexpNames()) {
		return nil, errors.New("snapshot URI could not be parsed")
	}

	snapshotID := &snapshotIdentifier{}

	// capture names start at index 1 to line up with the corresponding indexes
	// of submatches (see godoc on SubexpNames())
	for i, names := 1, snapshotURIRegexp.SubexpNames(); i < len(names); i++ {
		switch names[i] {
		case "subscription":
			snapshotID.subscription = submatches[i]
		case "resourceGroup":
			snapshotID.resourceGroup = submatches[i]
		case "snapshotName":
			snapshotID.name = submatches[i]
		default:
			return nil, errors.New("unexpected named capture from snapshot URI regex")
		}
	}

	return snapshotID, nil
}

func (b *VolumeSnapshotter) GetVolumeID(unstructuredPV runtime.Unstructured) (string, error) {
	pv := new(v1.PersistentVolume)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredPV.UnstructuredContent(), pv); err != nil {
		return "", errors.WithStack(err)
	}

	if pv.Spec.AzureDisk == nil {
		return "", nil
	}

	if pv.Spec.AzureDisk.DiskName == "" {
		return "", errors.New("spec.azureDisk.diskName not found")
	}

	return pv.Spec.AzureDisk.DiskName, nil
}

func (b *VolumeSnapshotter) SetVolumeID(unstructuredPV runtime.Unstructured, volumeID string) (runtime.Unstructured, error) {
	pv := new(v1.PersistentVolume)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredPV.UnstructuredContent(), pv); err != nil {
		return nil, errors.WithStack(err)
	}

	if pv.Spec.AzureDisk == nil {
		return nil, errors.New("spec.azureDisk not found")
	}

	pv.Spec.AzureDisk.DiskName = volumeID
	pv.Spec.AzureDisk.DataDiskURI = getComputeResourceName(b.subscription, b.disksResourceGroup, disksResource, volumeID)

	res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pv)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &unstructured.Unstructured{Object: res}, nil
}
