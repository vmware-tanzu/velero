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

package plugin

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/heptio/velero/pkg/backup"
	"github.com/heptio/velero/pkg/client"
	"github.com/heptio/velero/pkg/cloudprovider/aws"
	"github.com/heptio/velero/pkg/cloudprovider/azure"
	"github.com/heptio/velero/pkg/cloudprovider/gcp"
	velerodiscovery "github.com/heptio/velero/pkg/discovery"
	veleroplugin "github.com/heptio/velero/pkg/plugin/framework"
	"github.com/heptio/velero/pkg/restore"
)

func NewCommand(f client.Factory) *cobra.Command {
	pluginServer := veleroplugin.NewServer()
	c := &cobra.Command{
		Use:    "run-plugins",
		Hidden: true,
		Short:  "INTERNAL COMMAND ONLY - not intended to be run directly by users",
		Run: func(c *cobra.Command, args []string) {
			pluginServer.
				RegisterObjectStore("velero.io/aws", newAwsObjectStore).
				RegisterObjectStore("velero.io/azure", newAzureObjectStore).
				RegisterObjectStore("velero.io/gcp", newGcpObjectStore).
				RegisterVolumeSnapshotter("velero.io/aws", newAwsVolumeSnapshotter).
				RegisterVolumeSnapshotter("velero.io/azure", newAzureVolumeSnapshotter).
				RegisterVolumeSnapshotter("velero.io/gcp", newGcpVolumeSnapshotter).
				RegisterBackupItemAction("velero.io/pv", newPVBackupItemAction).
				RegisterBackupItemAction("velero.io/pod", newPodBackupItemAction).
				RegisterBackupItemAction("velero.io/serviceaccount", newServiceAccountBackupItemAction(f)).
				RegisterRestoreItemAction("velero.io/job", newJobRestoreItemAction).
				RegisterRestoreItemAction("velero.io/pod", newPodRestoreItemAction).
				RegisterRestoreItemAction("velero.io/restic", newResticRestoreItemAction(f)).
				RegisterRestoreItemAction("velero.io/service", newServiceRestoreItemAction).
				RegisterRestoreItemAction("velero.io/serviceaccount", newServiceAccountRestoreItemAction).
				RegisterRestoreItemAction("velero.io/addPVCFromPod", newAddPVCFromPodRestoreItemAction).
				RegisterRestoreItemAction("velero.io/addPVFromPVC", newAddPVFromPVCRestoreItemAction).
				Serve()
		},
	}

	pluginServer.BindFlags(c.Flags())

	return c
}

func newAwsObjectStore(logger logrus.FieldLogger) (interface{}, error) {
	return aws.NewObjectStore(logger), nil
}

func newAzureObjectStore(logger logrus.FieldLogger) (interface{}, error) {
	return azure.NewObjectStore(logger), nil
}

func newGcpObjectStore(logger logrus.FieldLogger) (interface{}, error) {
	return gcp.NewObjectStore(logger), nil
}

func newAwsVolumeSnapshotter(logger logrus.FieldLogger) (interface{}, error) {
	return aws.NewVolumeSnapshotter(logger), nil
}

func newAzureVolumeSnapshotter(logger logrus.FieldLogger) (interface{}, error) {
	return azure.NewVolumeSnapshotter(logger), nil
}

func newGcpVolumeSnapshotter(logger logrus.FieldLogger) (interface{}, error) {
	return gcp.NewVolumeSnapshotter(logger), nil
}

func newPVBackupItemAction(logger logrus.FieldLogger) (interface{}, error) {
	return backup.NewPVCAction(logger), nil
}

func newPodBackupItemAction(logger logrus.FieldLogger) (interface{}, error) {
	return backup.NewPodAction(logger), nil
}

func newServiceAccountBackupItemAction(f client.Factory) veleroplugin.HandlerInitializer {
	return func(logger logrus.FieldLogger) (interface{}, error) {
		// TODO(ncdc): consider a k8s style WantsKubernetesClientSet initialization approach
		clientset, err := f.KubeClient()
		if err != nil {
			return nil, err
		}

		discoveryHelper, err := velerodiscovery.NewHelper(clientset.Discovery(), logger)
		if err != nil {
			return nil, err
		}

		action, err := backup.NewServiceAccountAction(
			logger,
			backup.NewClusterRoleBindingListerMap(clientset),
			discoveryHelper)
		if err != nil {
			return nil, err
		}

		return action, nil
	}
}

func newJobRestoreItemAction(logger logrus.FieldLogger) (interface{}, error) {
	return restore.NewJobAction(logger), nil
}

func newPodRestoreItemAction(logger logrus.FieldLogger) (interface{}, error) {
	return restore.NewPodAction(logger), nil
}

func newResticRestoreItemAction(f client.Factory) veleroplugin.HandlerInitializer {
	return func(logger logrus.FieldLogger) (interface{}, error) {
		client, err := f.KubeClient()
		if err != nil {
			return nil, err
		}

		return restore.NewResticRestoreAction(logger, client.CoreV1().ConfigMaps(f.Namespace())), nil
	}
}

func newServiceRestoreItemAction(logger logrus.FieldLogger) (interface{}, error) {
	return restore.NewServiceAction(logger), nil
}

func newServiceAccountRestoreItemAction(logger logrus.FieldLogger) (interface{}, error) {
	return restore.NewServiceAccountAction(logger), nil
}

func newAddPVCFromPodRestoreItemAction(logger logrus.FieldLogger) (interface{}, error) {
	return restore.NewAddPVCFromPodAction(logger), nil
}

func newAddPVFromPVCRestoreItemAction(logger logrus.FieldLogger) (interface{}, error) {
	return restore.NewAddPVFromPVCAction(logger), nil
}
