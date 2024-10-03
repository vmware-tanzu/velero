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
	"strings"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/vmware-tanzu/velero/pkg/nodeagent"
)

type inheritedPodInfo struct {
	image          string
	serviceAccount string
	env            []v1.EnvVar
	volumeMounts   []v1.VolumeMount
	volumes        []v1.Volume
	logLevelArgs   []string
	logFormatArgs  []string
}

func getInheritedPodInfo(ctx context.Context, client kubernetes.Interface, veleroNamespace string) (inheritedPodInfo, error) {
	podInfo := inheritedPodInfo{}

	podSpec, err := nodeagent.GetPodSpec(ctx, client, veleroNamespace)
	if err != nil {
		return podInfo, errors.Wrap(err, "error to get node-agent pod template")
	}

	if len(podSpec.Containers) != 1 {
		return podInfo, errors.New("unexpected pod template from node-agent")
	}

	podInfo.image = podSpec.Containers[0].Image
	podInfo.serviceAccount = podSpec.ServiceAccountName

	podInfo.env = podSpec.Containers[0].Env
	podInfo.volumeMounts = podSpec.Containers[0].VolumeMounts
	podInfo.volumes = podSpec.Volumes

	args := podSpec.Containers[0].Args
	for i, arg := range args {
		if arg == "--log-format" {
			podInfo.logFormatArgs = append(podInfo.logFormatArgs, args[i:i+2]...)
		} else if strings.HasPrefix(arg, "--log-format") {
			podInfo.logFormatArgs = append(podInfo.logFormatArgs, arg)
		} else if arg == "--log-level" {
			podInfo.logLevelArgs = append(podInfo.logLevelArgs, args[i:i+2]...)
		} else if strings.HasPrefix(arg, "--log-level") {
			podInfo.logLevelArgs = append(podInfo.logLevelArgs, arg)
		}
	}

	return podInfo, nil
}
