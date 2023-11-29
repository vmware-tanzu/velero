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

package metrics

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

func GetPodUsageMetrics(ctx context.Context, metricsClient *metricsclientset.Clientset, podName, namespace string, podMetricsTimeout time.Duration) (cpuUsage, memoryUsage resource.Quantity, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), podMetricsTimeout)
	defer cancel()

	var podMetrics *v1beta1.PodMetrics
	err = wait.PollImmediateUntil(time.Second, func() (bool, error) {
		var err error
		podMetrics, err = metricsClient.MetricsV1beta1().PodMetricses(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return true, nil
	}, ctx.Done())

	if err != nil {
		return
	} else if podMetrics == nil {
		return cpuUsage, memoryUsage, nil
	}
	// Variables to store the max and sum of CPU and memory usage
	// For velero pod we only return the main container
	for _, container := range podMetrics.Containers {
		cpuUsage = container.Usage[corev1.ResourceCPU]
		memoryUsage = container.Usage[corev1.ResourceMemory]
		return
	}

	return
}
