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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

type exitWithMessageMock struct {
	createErr error
	writeFail bool
	filePath  string
	exitCode  int
}

func (em *exitWithMessageMock) Exit(code int) {
	em.exitCode = code
}

func (em *exitWithMessageMock) CreateFile(_ string) (*os.File, error) {
	if em.createErr != nil {
		return nil, em.createErr
	}

	if em.writeFail {
		return os.OpenFile(em.filePath, os.O_CREATE|os.O_RDONLY, 0500)
	} else {
		return os.Create(em.filePath)
	}
}

func TestExitWithMessage(t *testing.T) {
	tests := []struct {
		name             string
		message          string
		succeed          bool
		args             []any
		createErr        error
		writeFail        bool
		expectedExitCode int
		expectedMessage  string
	}{
		{
			name:             "create pod file failed",
			createErr:        errors.New("fake-create-file-error"),
			succeed:          true,
			expectedExitCode: 1,
		},
		{
			name:             "write pod file failed",
			writeFail:        true,
			succeed:          true,
			expectedExitCode: 1,
		},
		{
			name:    "not succeed",
			message: "fake-message-1, arg-1 %s, arg-2 %v, arg-3 %v",
			args: []any{
				"arg-1-1",
				10,
				false,
			},
			expectedExitCode: 1,
			expectedMessage:  fmt.Sprintf("fake-message-1, arg-1 %s, arg-2 %v, arg-3 %v", "arg-1-1", 10, false),
		},
		{
			name:    "not succeed",
			message: "fake-message-2, arg-1 %s, arg-2 %v, arg-3 %v",
			args: []any{
				"arg-1-2",
				20,
				true,
			},
			succeed:         true,
			expectedMessage: fmt.Sprintf("fake-message-2, arg-1 %s, arg-2 %v, arg-3 %v", "arg-1-2", 20, true),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			podFile := filepath.Join(os.TempDir(), uuid.NewString())

			em := exitWithMessageMock{
				createErr: test.createErr,
				writeFail: test.writeFail,
				filePath:  podFile,
			}

			funcExit = em.Exit
			funcCreateFile = em.CreateFile

			exitWithMessage(velerotest.NewLogger(), test.succeed, test.message, test.args...)

			assert.Equal(t, test.expectedExitCode, em.exitCode)

			if test.createErr == nil && !test.writeFail {
				reader, err := os.Open(podFile)
				require.NoError(t, err)

				message, err := io.ReadAll(reader)
				require.NoError(t, err)

				reader.Close()

				assert.Equal(t, test.expectedMessage, string(message))
			}
		})
	}
}
