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

package kube

import (
	"context"
	"fmt"

	appsv1api "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/pkg/util/velero"
)

// GetVeleroDeployment returns a Velero deployment object, selected either by a name or a label selector
// if the former is not found (see https://github.com/vmware-tanzu/velero/issues/9023)
// and then by the container name (see https://github.com/vmware-tanzu/velero/issues/3961).
func GetVeleroDeployment(ctx context.Context, crClient ctrlclient.Client, namespace string) (*appsv1api.Deployment, error) {
	deployment := new(appsv1api.Deployment)
	if err := crClient.Get(ctx, types.NamespacedName{Name: "velero", Namespace: namespace}, deployment); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}

		// fallback to avoid #9023
		var (
			deployments = new(appsv1api.DeploymentList)
			ls          = labels.SelectorFromValidatedSet(velero.Labels())
		)
		if err := crClient.List(ctx, deployments, &ctrlclient.ListOptions{
			LabelSelector: ls,
			Limit:         1,
		}); err != nil {
			return nil, err
		}

		if len(deployments.Items) == 0 {
			return nil, fmt.Errorf("could not find velero deployment by the name nor by a label selector %q", ls)
		}

		deployment = &deployments.Items[0]
	}

	for _, c := range deployment.Spec.Template.Spec.Containers {
		if c.Name == "velero" {
			return deployment, nil
		}
	}

	return nil, fmt.Errorf("velero deployment not found")
}
