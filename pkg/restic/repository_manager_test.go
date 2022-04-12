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

package restic

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

func TestGetInsecureSkipTLSVerifyFromBSL(t *testing.T) {
	log := logrus.StandardLogger()
	tests := []struct {
		name           string
		backupLocation *velerov1api.BackupStorageLocation
		logger         logrus.FieldLogger
		expected       string
	}{
		{
			"Test with nil BSL. Should return empty string.",
			nil,
			log,
			"",
		},
		{
			"Test with none AWS BSL. Should return empty string.",
			&velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "azure",
				},
			},
			log,
			"",
		},
		{
			"Test with AWS BSL's insecureSkipTLSVerify set to false.",
			&velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "aws",
					Config: map[string]string{
						"insecureSkipTLSVerify": "false",
					},
				},
			},
			log,
			"--insecure-tls=false",
		},
		{
			"Test with AWS BSL's insecureSkipTLSVerify set to true.",
			&velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "aws",
					Config: map[string]string{
						"insecureSkipTLSVerify": "true",
					},
				},
			},
			log,
			"--insecure-tls=true",
		},
		{
			"Test with AWS BSL's insecureSkipTLSVerify set to invalid.",
			&velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "aws",
					Config: map[string]string{
						"insecureSkipTLSVerify": "invalid",
					},
				},
			},
			log,
			"",
		},
		{
			"Test with AWS without insecureSkipTLSVerify.",
			&velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "aws",
					Config:   map[string]string{},
				},
			},
			log,
			"",
		},
		{
			"Test with AWS without config.",
			&velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "aws",
				},
			},
			log,
			"",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res := getInsecureSkipTLSVerifyFromBSL(test.backupLocation, test.logger)

			assert.Equal(t, test.expected, res)
		})
	}
}
