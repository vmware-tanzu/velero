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

package plugin

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/heptio/ark/pkg/backup"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cloudprovider/aws"
	"github.com/heptio/ark/pkg/cloudprovider/azure"
	"github.com/heptio/ark/pkg/cloudprovider/gcp"
	arkplugin "github.com/heptio/ark/pkg/plugin"
	"github.com/heptio/ark/pkg/restore"
)

func NewCommand(f client.Factory) *cobra.Command {
	logger := arkplugin.NewLogger()

	c := &cobra.Command{
		Use:    "run-plugin",
		Hidden: true,
		Short:  "INTERNAL COMMAND ONLY - not intended to be run directly by users",
		Run: func(c *cobra.Command, args []string) {
			logger.Debug("Executing run-plugin command")

			arkplugin.NewServer(logger).
				RegisterObjectStore("aws", newAwsObjectStore).
				RegisterObjectStore("azure", newAzureObjectStore).
				RegisterObjectStore("gcp", newGcpObjectStore).
				RegisterBlockStore("aws", newAwsBlockStore).
				RegisterBlockStore("azure", newAzureBlockStore).
				RegisterBlockStore("gcp", newGcpBlockStore).
				RegisterBackupItemAction("pv", newPVBackupItemAction).
				RegisterBackupItemAction("pod", newPodBackupItemAction).
				RegisterBackupItemAction("serviceaccount", newServiceAccountBackupItemAction(f)).
				RegisterRestoreItemAction("job", newJobRestoreItemAction).
				RegisterRestoreItemAction("pod", newPodRestoreItemAction).
				RegisterRestoreItemAction("restic", newResticRestoreItemAction).
				RegisterRestoreItemAction("service", newServiceRestoreItemAction).
				Serve()
		},
	}

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

func newAwsBlockStore(logger logrus.FieldLogger) (interface{}, error) {
	return aws.NewBlockStore(logger), nil
}

func newAzureBlockStore(logger logrus.FieldLogger) (interface{}, error) {
	return azure.NewBlockStore(logger), nil
}

func newGcpBlockStore(logger logrus.FieldLogger) (interface{}, error) {
	return gcp.NewBlockStore(logger), nil
}

func newPVBackupItemAction(logger logrus.FieldLogger) (interface{}, error) {
	return backup.NewBackupPVAction(logger), nil
}

func newPodBackupItemAction(logger logrus.FieldLogger) (interface{}, error) {
	return backup.NewPodAction(logger), nil
}

func newServiceAccountBackupItemAction(f client.Factory) arkplugin.HandlerInitializer {
	return func(logger logrus.FieldLogger) (interface{}, error) {
		// TODO(ncdc): consider a k8s style WantsKubernetesClientSet initialization approach
		clientset, err := f.KubeClient()
		if err != nil {
			return nil, err
		}

		action, err := backup.NewServiceAccountAction(logger, clientset.RbacV1().ClusterRoleBindings())
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

func newResticRestoreItemAction(logger logrus.FieldLogger) (interface{}, error) {
	return restore.NewResticRestoreAction(logger), nil
}

func newServiceRestoreItemAction(logger logrus.FieldLogger) (interface{}, error) {
	return restore.NewServiceAction(logger), nil
}
