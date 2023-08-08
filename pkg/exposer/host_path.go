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
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/pkg/datapath"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
)

var getVolumeDirectory = kube.GetVolumeDirectory
var singlePathMatch = kube.SinglePathMatch

// GetPodVolumeHostPath returns a path that can be accessed from the host for a given volume of a pod
func GetPodVolumeHostPath(ctx context.Context, pod *corev1.Pod, volumeName string,
	cli ctrlclient.Client, fs filesystem.Interface, log logrus.FieldLogger) (datapath.AccessPoint, error) {
	logger := log.WithField("pod name", pod.Name).WithField("pod UID", pod.GetUID()).WithField("volume", volumeName)

	volDir, err := getVolumeDirectory(ctx, logger, pod, volumeName, cli)
	if err != nil {
		return datapath.AccessPoint{}, errors.Wrapf(err, "error getting volume directory name for volume %s in pod %s", volumeName, pod.Name)
	}

	logger.WithField("volDir", volDir).Info("Got volume dir")

	pathGlob := fmt.Sprintf("/host_pods/%s/volumes/*/%s", string(pod.GetUID()), volDir)
	logger.WithField("pathGlob", pathGlob).Debug("Looking for path matching glob")

	path, err := singlePathMatch(pathGlob, fs, logger)
	if err != nil {
		return datapath.AccessPoint{}, errors.Wrapf(err, "error identifying unique volume path on host for volume %s in pod %s", volumeName, pod.Name)
	}

	logger.WithField("path", path).Info("Found path matching glob")

	return datapath.AccessPoint{
		ByPath: path,
	}, nil
}
