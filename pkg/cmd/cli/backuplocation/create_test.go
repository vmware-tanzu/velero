/*
Copyright the Velero Contributors.

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

package backuplocation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildBackupStorageLocationSetsNamespace(t *testing.T) {
	o := NewCreateOptions()

	bsl, err := o.BuildBackupStorageLocation("velero-test-ns", false, false)
	assert.NoError(t, err)
	assert.Equal(t, "velero-test-ns", bsl.Namespace)
}

func TestBuildBackupStorageLocationSetsSyncPeriod(t *testing.T) {
	o := NewCreateOptions()
	o.BackupSyncPeriod = 2 * time.Minute

	bsl, err := o.BuildBackupStorageLocation("velero-test-ns", false, false)
	assert.NoError(t, err)
	assert.Nil(t, bsl.Spec.BackupSyncPeriod)

	bsl, err = o.BuildBackupStorageLocation("velero-test-ns", true, false)
	assert.NoError(t, err)
	assert.Equal(t, &metav1.Duration{Duration: 2 * time.Minute}, bsl.Spec.BackupSyncPeriod)
}

func TestBuildBackupStorageLocationSetsValidationFrequency(t *testing.T) {
	o := NewCreateOptions()
	o.ValidationFrequency = 2 * time.Minute

	bsl, err := o.BuildBackupStorageLocation("velero-test-ns", false, false)
	assert.NoError(t, err)
	assert.Nil(t, bsl.Spec.ValidationFrequency)

	bsl, err = o.BuildBackupStorageLocation("velero-test-ns", false, true)
	assert.NoError(t, err)
	assert.Equal(t, &metav1.Duration{Duration: 2 * time.Minute}, bsl.Spec.ValidationFrequency)
}

func TestBuildBackupStorageLocationSetsCredential(t *testing.T) {
	o := NewCreateOptions()

	bsl, err := o.BuildBackupStorageLocation("velero-test-ns", false, false)
	assert.NoError(t, err)
	assert.Nil(t, bsl.Spec.Credential)

	setErr := o.Credential.Set("my-secret=key-from-secret")
	assert.NoError(t, setErr)

	bsl, err = o.BuildBackupStorageLocation("velero-test-ns", false, true)
	assert.NoError(t, err)
	assert.Equal(t, &v1.SecretKeySelector{
		LocalObjectReference: v1.LocalObjectReference{Name: "my-secret"},
		Key:                  "key-from-secret",
	}, bsl.Spec.Credential)
}

func TestBuildBackupStorageLocationSetsLabels(t *testing.T) {
	o := NewCreateOptions()

	err := o.Labels.Set("key=value")
	assert.NoError(t, err)

	bsl, err := o.BuildBackupStorageLocation("velero-test-ns", false, false)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"key": "value"}, bsl.Labels)
}
