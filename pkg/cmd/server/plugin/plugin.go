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
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/heptio/velero/pkg/backup"
	"github.com/heptio/velero/pkg/client"
	"github.com/heptio/velero/pkg/cloudprovider/aws"
	"github.com/heptio/velero/pkg/cloudprovider/azure"
	"github.com/heptio/velero/pkg/cloudprovider/gcp"
	velerodiscovery "github.com/heptio/velero/pkg/discovery"
	veleroplugin "github.com/heptio/velero/pkg/plugin"
	"github.com/heptio/velero/pkg/restore"
	"github.com/heptio/velero/pkg/util/logging"
)

func NewCommand(f client.Factory) *cobra.Command {
	logLevelFlag := logging.LogLevelFlag(logrus.InfoLevel)

	c := &cobra.Command{
		Use:    "run-plugins",
		Hidden: true,
		Short:  "INTERNAL COMMAND ONLY - not intended to be run directly by users",
		Run: func(c *cobra.Command, args []string) {
			logLevel := logLevelFlag.Parse()
			logrus.Infof("Setting log-level to %s", strings.ToUpper(logLevel.String()))

			logger := veleroplugin.NewLogger(logLevel)

			logger.Debug("Executing run-plugins command")

			veleroplugin.NewServer(logger).
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
				RegisterRestoreItemAction("serviceaccount", newServiceAccountRestoreItemAction).
				Serve()
		},
	}

	c.Flags().Var(logLevelFlag, "log-level", fmt.Sprintf("the level at which to log. Valid values are %s.", strings.Join(logLevelFlag.AllowedValues(), ", ")))

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

func newResticRestoreItemAction(logger logrus.FieldLogger) (interface{}, error) {
	return restore.NewResticRestoreAction(logger), nil
}

func newServiceRestoreItemAction(logger logrus.FieldLogger) (interface{}, error) {
	return restore.NewServiceAction(logger), nil
}

func newServiceAccountRestoreItemAction(logger logrus.FieldLogger) (interface{}, error) {
	return restore.NewServiceAccountAction(logger), nil
}
