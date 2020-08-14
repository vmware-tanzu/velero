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
	"fmt"
	"time"

	"github.com/pkg/errors"
	"k8s.io/client-go/tools/cache"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	velerov1client "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/typed/velero/v1"
)

type ServerStatusGetter interface {
	GetServerStatus(client velerov1client.ServerStatusRequestsGetter) (*velerov1api.ServerStatusRequest, error)
}

// select stmt waiting for a time out or a response (data on the channel)
type DefaultServerStatusGetter struct {
	Namespace string
	Timeout   time.Duration
}

func (g *DefaultServerStatusGetter) GetServerStatus(mgr manager.Manager) (*velerov1api.ServerStatusRequest, error) {
	req := builder.ForServerStatusRequest(g.Namespace, "", "0").
		ObjectMeta(
			builder.WithGenerateName("velero-cli-"),
		).Result()

	// create an informer for the target resource
	reqInformer, err := mgr.GetCache().GetInformer(context.TODO(), req)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	out := make(chan *velerov1api.ServerStatusRequest, 1)
	defer close(out)
	updateFunc := func(_, newObj interface{}) {
		var statusReq *velerov1api.ServerStatusRequest
		defer func() {
			out <- statusReq
		}()

		// inside the updateFunc, once the resource has the terminal state you're looking for, send that object to a channel
		// (create that channel before you create the informer,
		// and receive from the channel after the informer has been configured and you want to use the value)

		statusReq, ok := newObj.(*velerov1api.ServerStatusRequest)
		if !ok {
			fmt.Printf("unexpected type %+v", newObj)
			return
		}

		fmt.Printf("\n\nUPDATED NEW %+v", statusReq)

		// if statusReq.Status.Phase == velerov1api.ServerStatusRequestPhaseProcessed {
		// req = statusReq
		// }
	}

	fmt.Printf("ss before: %+v\n\n", req)

	reqInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: updateFunc,
	})

	if err := mgr.GetClient().Create(context.TODO(), req, &kbclient.CreateOptions{}); err != nil {
		return nil, errors.WithStack(err)
	}

	timeOut := 10 * time.Second
	expired := time.NewTimer(timeOut)
	defer expired.Stop()

Loop:
	for {
		select {
		case statusReq := <-out:
			fmt.Printf("\n\nss after but inside select: %+v", statusReq)
			req = statusReq
			break Loop
		case <-expired.C:
			fmt.Println("time out!!!!")
			return nil, errors.New("timed out waiting for server status request to be processed")
		}
	}

	fmt.Printf("\n\nss after: %+v", req)

	return req, nil
}
