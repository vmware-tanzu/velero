/*
Copyright The Velero Contributors.
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

package datamover

import (
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/vmware-tanzu/velero/pkg/buildinfo"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/util/logging"
)

type dataMoverBackupConfig struct {
	volumePath      string
	volumeMode      string
	duName          string
	resourceTimeout time.Duration
}

func NewBackupCommand(f client.Factory) *cobra.Command {
	config := dataMoverBackupConfig{}

	logLevelFlag := logging.LogLevelFlag(logrus.InfoLevel)
	formatFlag := logging.NewFormatFlag()

	command := &cobra.Command{
		Use:    "backup",
		Short:  "Run the velero data-mover backup",
		Long:   "Run the velero data-mover backup",
		Hidden: true,
		Run: func(c *cobra.Command, args []string) {
			logLevel := logLevelFlag.Parse()
			logrus.Infof("Setting log-level to %s", strings.ToUpper(logLevel.String()))

			logger := logging.DefaultLogger(logLevel, formatFlag.Parse())
			logger.Infof("Starting Velero data-mover backup %s (%s)", buildinfo.Version, buildinfo.FormattedGitSHA())

			f.SetBasename(fmt.Sprintf("%s-%s", c.Parent().Name(), c.Name()))
			s := newdataMoverBackup(logger, config)

			s.run()
		},
	}

	command.Flags().Var(logLevelFlag, "log-level", fmt.Sprintf("The level at which to log. Valid values are %s.", strings.Join(logLevelFlag.AllowedValues(), ", ")))
	command.Flags().Var(formatFlag, "log-format", fmt.Sprintf("The format for log output. Valid values are %s.", strings.Join(formatFlag.AllowedValues(), ", ")))
	command.Flags().StringVar(&config.volumePath, "volume-path", config.volumePath, "The full path of the volume to be backed up")
	command.Flags().StringVar(&config.volumeMode, "volume-mode", config.volumeMode, "The mode of the volume to be backed up")
	command.Flags().StringVar(&config.duName, "data-upload", config.duName, "The data upload name")
	command.Flags().DurationVar(&config.resourceTimeout, "resource-timeout", config.resourceTimeout, "How long to wait for resource processes which are not covered by other specific timeout parameters.")

	_ = command.MarkFlagRequired("volume-path")
	_ = command.MarkFlagRequired("volume-mode")
	_ = command.MarkFlagRequired("data-upload")
	_ = command.MarkFlagRequired("resource-timeout")

	return command
}

type dataMoverBackup struct {
	logger logrus.FieldLogger
	config dataMoverBackupConfig
}

func newdataMoverBackup(logger logrus.FieldLogger, config dataMoverBackupConfig) *dataMoverBackup {
	s := &dataMoverBackup{
		logger: logger,
		config: config,
	}

	return s
}

func (s *dataMoverBackup) run() {
	time.Sleep(time.Duration(1<<63 - 1))
}
