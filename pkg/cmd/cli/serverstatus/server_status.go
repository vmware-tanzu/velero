/*
Copyright 2020 the Velero contributors.

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

package serverstatus

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
)

type ServerStatusGetter interface {
	GetServerStatus(kbClient kbclient.Client) (*velerov1api.ServerStatusRequest, error)
}

type DefaultServerStatusGetter struct {
	Namespace string
	Context   context.Context
}

func (g *DefaultServerStatusGetter) GetServerStatus(kbClient kbclient.Client) (*velerov1api.ServerStatusRequest, error) {
	created := builder.ForServerStatusRequest(g.Namespace, "", "0").ObjectMeta(builder.WithGenerateName("velero-cli-")).Result()

	if err := kbClient.Create(context.Background(), created, &kbclient.CreateOptions{}); err != nil {
		return nil, errors.WithStack(err)
	}

	ctx, cancel := context.WithCancel(g.Context)
	defer cancel()

	key := client.ObjectKey{Name: created.Name, Namespace: g.Namespace}
	checkFunc := func() {
		updated := &velerov1api.ServerStatusRequest{}
		if err := kbClient.Get(ctx, key, updated); err != nil {
			return
		}

		// TODO: once the minimum supported Kubernetes version is v1.9.0, remove the following check.
		// See http://issue.k8s.io/51046 for details.
		if updated.Name != created.Name {
			return
		}

		if updated.Status.Phase == velerov1api.ServerStatusRequestPhaseProcessed {
			created = updated
			cancel()
		}
	}

	wait.Until(checkFunc, 250*time.Millisecond, ctx.Done())

	return created, nil
}
