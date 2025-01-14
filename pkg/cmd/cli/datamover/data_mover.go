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
	"context"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/vmware-tanzu/velero/pkg/client"
)

func NewCommand(f client.Factory) *cobra.Command {
	command := &cobra.Command{
		Use:    "data-mover",
		Short:  "Run the velero data-mover",
		Long:   "Run the velero data-mover",
		Hidden: true,
	}

	command.AddCommand(
		NewBackupCommand(f),
		NewRestoreCommand(f),
	)

	return command
}

type dataPathService interface {
	Init() error
	RunCancelableDataPath(context.Context) (string, error)
	Shutdown()
}

var (
	funcExit       = os.Exit
	funcCreateFile = os.Create
)

func exitWithMessage(logger logrus.FieldLogger, succeed bool, message string, a ...any) {
	exitCode := 0
	if !succeed {
		exitCode = 1
	}

	toWrite := fmt.Sprintf(message, a...)

	podFile, err := funcCreateFile("/dev/termination-log")
	if err != nil {
		logger.WithError(err).Error("Failed to create termination log file")
		exitCode = 1
	} else {
		if _, err := podFile.WriteString(toWrite); err != nil {
			logger.WithError(err).Error("Failed to write error to termination log file")
			exitCode = 1
		}

		podFile.Close()
	}

	funcExit(exitCode)
}
