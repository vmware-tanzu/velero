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

package exposer

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/uploader"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
)

func TestGetPodVolumeHostPath(t *testing.T) {
	tests := []struct {
		name              string
		getVolumeDirFunc  func(context.Context, logrus.FieldLogger, *corev1.Pod, string, ctrlclient.Client) (string, error)
		getVolumeModeFunc func(context.Context, logrus.FieldLogger, *corev1.Pod, string, ctrlclient.Client) (uploader.PersistentVolumeMode, error)
		pathMatchFunc     func(string, filesystem.Interface, logrus.FieldLogger) (string, error)
		pod               *corev1.Pod
		pvc               string
		err               string
	}{
		{
			name: "get volume dir fail",
			getVolumeDirFunc: func(context.Context, logrus.FieldLogger, *corev1.Pod, string, ctrlclient.Client) (string, error) {
				return "", errors.New("fake-error-1")
			},
			pod: builder.ForPod(velerov1api.DefaultNamespace, "fake-pod-1").Result(),
			pvc: "fake-pvc-1",
			err: "error getting volume directory name for volume fake-pvc-1 in pod fake-pod-1: fake-error-1",
		},
		{
			name: "single path match fail",
			getVolumeDirFunc: func(context.Context, logrus.FieldLogger, *corev1.Pod, string, ctrlclient.Client) (string, error) {
				return "", nil
			},
			getVolumeModeFunc: func(context.Context, logrus.FieldLogger, *corev1.Pod, string, ctrlclient.Client) (uploader.PersistentVolumeMode, error) {
				return uploader.PersistentVolumeFilesystem, nil
			},
			pathMatchFunc: func(string, filesystem.Interface, logrus.FieldLogger) (string, error) {
				return "", errors.New("fake-error-2")
			},
			pod: builder.ForPod(velerov1api.DefaultNamespace, "fake-pod-2").Result(),
			pvc: "fake-pvc-1",
			err: "error identifying unique volume path on host for volume fake-pvc-1 in pod fake-pod-2: fake-error-2",
		},
		{
			name: "get block volume dir success",
			getVolumeDirFunc: func(context.Context, logrus.FieldLogger, *corev1.Pod, string, ctrlclient.Client) (
				string, error) {
				return "fake-pvc-1", nil
			},
			pathMatchFunc: func(string, filesystem.Interface, logrus.FieldLogger) (string, error) {
				return "/host_pods/fake-pod-1-id/volumeDevices/kubernetes.io~csi/fake-pvc-1-id", nil
			},
			pod: builder.ForPod(velerov1api.DefaultNamespace, "fake-pod-1").Result(),
			pvc: "fake-pvc-1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.getVolumeDirFunc != nil {
				getVolumeDirectory = test.getVolumeDirFunc
			}

			if test.getVolumeModeFunc != nil {
				getVolumeMode = test.getVolumeModeFunc
			}

			if test.pathMatchFunc != nil {
				singlePathMatch = test.pathMatchFunc
			}

			_, err := GetPodVolumeHostPath(context.Background(), test.pod, test.pvc, nil, nil, velerotest.NewLogger())
			if test.err != "" || err != nil {
				assert.EqualError(t, err, test.err)
			}
		})
	}
}
