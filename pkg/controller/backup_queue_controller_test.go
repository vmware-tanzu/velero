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

package controller

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

func TestBackupQueueReconciler(t *testing.T) {
	scheme := runtime.NewScheme()
	velerov1api.AddToScheme(scheme)

	backup := &velerov1api.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "velero",
			Name:      "test-backup",
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(backup).Build()
	logger := logrus.New()
	log := logger.WithField("controller", "backup-queue-test")

	r := NewBackupQueueReconciler(fakeClient, scheme, log)
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "velero",
			Name:      "test-backup",
		},
	}

	res, err := r.Reconcile(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, res)
}
