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

type dataMoverRestoreConfig struct {
	volumePath      string
	volumeMode      string
	ddName          string
	resourceTimeout time.Duration
}

func NewRestoreCommand(f client.Factory) *cobra.Command {
	logLevelFlag := logging.LogLevelFlag(logrus.InfoLevel)
	formatFlag := logging.NewFormatFlag()

	config := dataMoverRestoreConfig{}

	command := &cobra.Command{
		Use:    "restore",
		Short:  "Run the velero data-mover restore",
		Long:   "Run the velero data-mover restore",
		Hidden: true,
		Run: func(c *cobra.Command, args []string) {
			logLevel := logLevelFlag.Parse()
			logrus.Infof("Setting log-level to %s", strings.ToUpper(logLevel.String()))

			logger := logging.DefaultLogger(logLevel, formatFlag.Parse())
			logger.Infof("Starting Velero data-mover restore %s (%s)", buildinfo.Version, buildinfo.FormattedGitSHA())

			f.SetBasename(fmt.Sprintf("%s-%s", c.Parent().Name(), c.Name()))
			s := newdataMoverRestore(logger, config)

			s.run()
		},
	}

	command.Flags().Var(logLevelFlag, "log-level", fmt.Sprintf("The level at which to log. Valid values are %s.", strings.Join(logLevelFlag.AllowedValues(), ", ")))
	command.Flags().Var(formatFlag, "log-format", fmt.Sprintf("The format for log output. Valid values are %s.", strings.Join(formatFlag.AllowedValues(), ", ")))
	command.Flags().StringVar(&config.volumePath, "volume-path", config.volumePath, "The full path of the volume to be restored")
	command.Flags().StringVar(&config.volumeMode, "volume-mode", config.volumeMode, "The mode of the volume to be restored")
	command.Flags().StringVar(&config.ddName, "data-download", config.ddName, "The data download name")
	command.Flags().DurationVar(&config.resourceTimeout, "resource-timeout", config.resourceTimeout, "How long to wait for resource processes which are not covered by other specific timeout parameters.")

	_ = command.MarkFlagRequired("volume-path")
	_ = command.MarkFlagRequired("volume-mode")
	_ = command.MarkFlagRequired("data-download")
	_ = command.MarkFlagRequired("resource-timeout")

	return command
}

type dataMoverRestore struct {
	logger logrus.FieldLogger
	config dataMoverRestoreConfig
}

func newdataMoverRestore(logger logrus.FieldLogger, config dataMoverRestoreConfig) *dataMoverRestore {
	s := &dataMoverRestore{
		logger: logger,
		config: config,
	}

	return s
}

func (s *dataMoverRestore) run() {
	time.Sleep(time.Duration(1<<63 - 1))
}
