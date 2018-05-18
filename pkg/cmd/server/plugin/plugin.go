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

			arkplugin.NewSimpleServer(logger).
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
				RegisterRestoreItemAction("service", newServiceRestoreItemAction).
				Serve()
		},
	}

	return c
}

func newAwsObjectStore() (interface{}, error) {
	return aws.NewObjectStore(), nil
}

func newAzureObjectStore() (interface{}, error) {
	return azure.NewObjectStore(), nil
}

func newGcpObjectStore() (interface{}, error) {
	return gcp.NewObjectStore(), nil
}

func newAwsBlockStore() (interface{}, error) {
	return aws.NewBlockStore(), nil
}

func newAzureBlockStore() (interface{}, error) {
	return azure.NewBlockStore(), nil
}

func newGcpBlockStore() (interface{}, error) {
	return gcp.NewBlockStore(), nil
}

func newPVBackupItemAction() (interface{}, error) {
	return backup.NewBackupPVAction(), nil
}

func newPodBackupItemAction() (interface{}, error) {
	return backup.NewPodAction(), nil
}

func newServiceAccountBackupItemAction(f client.Factory) arkplugin.ServerImplFactory {
	return func() (interface{}, error) {
		// TODO(ncdc): consider a k8s style WantsKubernetesClientSet initialization approach
		clientset, err := f.KubeClient()
		if err != nil {
			return nil, err
		}

		action, err := backup.NewServiceAccountAction(clientset.RbacV1().ClusterRoleBindings())
		if err != nil {
			return nil, err
		}

		return action, nil
	}
}

func newJobRestoreItemAction() (interface{}, error) {
	return restore.NewJobAction(), nil
}

func newPodRestoreItemAction() (interface{}, error) {
	return restore.NewPodAction(), nil
}

func newServiceRestoreItemAction() (interface{}, error) {
	return restore.NewServiceAction(), nil
}
