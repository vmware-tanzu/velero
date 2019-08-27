/*
Copyright the Velero contributors.

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

// Code generated by client-gen. DO NOT EDIT.

package v1

import (
	v1 "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/generated/clientset/versioned/scheme"
	rest "k8s.io/client-go/rest"
)

type VeleroV1Interface interface {
	RESTClient() rest.Interface
	BackupsGetter
	BackupStorageLocationsGetter
	DeleteBackupRequestsGetter
	DownloadRequestsGetter
	PodVolumeBackupsGetter
	PodVolumeRestoresGetter
	ResticRepositoriesGetter
	RestoresGetter
	SchedulesGetter
	ServerStatusRequestsGetter
	VolumeSnapshotLocationsGetter
}

// VeleroV1Client is used to interact with features provided by the velero.io group.
type VeleroV1Client struct {
	restClient rest.Interface
}

func (c *VeleroV1Client) Backups(namespace string) BackupInterface {
	return newBackups(c, namespace)
}

func (c *VeleroV1Client) BackupStorageLocations(namespace string) BackupStorageLocationInterface {
	return newBackupStorageLocations(c, namespace)
}

func (c *VeleroV1Client) DeleteBackupRequests(namespace string) DeleteBackupRequestInterface {
	return newDeleteBackupRequests(c, namespace)
}

func (c *VeleroV1Client) DownloadRequests(namespace string) DownloadRequestInterface {
	return newDownloadRequests(c, namespace)
}

func (c *VeleroV1Client) PodVolumeBackups(namespace string) PodVolumeBackupInterface {
	return newPodVolumeBackups(c, namespace)
}

func (c *VeleroV1Client) PodVolumeRestores(namespace string) PodVolumeRestoreInterface {
	return newPodVolumeRestores(c, namespace)
}

func (c *VeleroV1Client) ResticRepositories(namespace string) ResticRepositoryInterface {
	return newResticRepositories(c, namespace)
}

func (c *VeleroV1Client) Restores(namespace string) RestoreInterface {
	return newRestores(c, namespace)
}

func (c *VeleroV1Client) Schedules(namespace string) ScheduleInterface {
	return newSchedules(c, namespace)
}

func (c *VeleroV1Client) ServerStatusRequests(namespace string) ServerStatusRequestInterface {
	return newServerStatusRequests(c, namespace)
}

func (c *VeleroV1Client) VolumeSnapshotLocations(namespace string) VolumeSnapshotLocationInterface {
	return newVolumeSnapshotLocations(c, namespace)
}

// NewForConfig creates a new VeleroV1Client for the given config.
func NewForConfig(c *rest.Config) (*VeleroV1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &VeleroV1Client{client}, nil
}

// NewForConfigOrDie creates a new VeleroV1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *VeleroV1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new VeleroV1Client for the given RESTClient.
func New(c rest.Interface) *VeleroV1Client {
	return &VeleroV1Client{c}
}

func setConfigDefaults(config *rest.Config) error {
	gv := v1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *VeleroV1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
