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

package test

import (
	"testing"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	"github.com/stretchr/testify/require"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

func NewFakeControllerRuntimeClientBuilder(t *testing.T) *k8sfake.ClientBuilder {
	scheme := runtime.NewScheme()
	err := velerov1api.AddToScheme(scheme)
	require.NoError(t, err)
	err = corev1api.AddToScheme(scheme)
	require.NoError(t, err)
	err = snapshotv1api.AddToScheme(scheme)
	require.NoError(t, err)
	return k8sfake.NewClientBuilder().WithScheme(scheme)
}

func NewFakeControllerRuntimeClient(t *testing.T, initObjs ...runtime.Object) client.Client {
	scheme := runtime.NewScheme()
	err := velerov1api.AddToScheme(scheme)
	require.NoError(t, err)
	err = corev1api.AddToScheme(scheme)
	require.NoError(t, err)
	err = snapshotv1api.AddToScheme(scheme)
	require.NoError(t, err)
	return k8sfake.NewFakeClientWithScheme(scheme, initObjs...)
}
