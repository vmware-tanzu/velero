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
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"
	"k8s.io/client-go/tools/cache"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

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

	out := make(chan interface{}, 1)
	defer close(out)

	addFunc := func(obj interface{}) {
		fmt.Println("\n\ninside addFunc *** ")
		out <- obj
	}

	updateFunc := func(_, newObj interface{}) {
		fmt.Println("\n\ninside updateFunc ### ")
		out <- newObj
	}

	reqInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    addFunc,
		UpdateFunc: updateFunc,
	})
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	wg.Add(1)
	go func() {
		defer wg.Done()

		if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
			fmt.Println(err, "unable to continue running manager")
			cancel()
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	if err := mgr.GetClient().Create(context.TODO(), req, &kbclient.CreateOptions{}); err != nil {
		return nil, errors.WithStack(err)
	}

	timeOut := 10 * time.Second
	expired := time.NewTimer(timeOut)
	defer expired.Stop()

Loop:
	for {
		select {
		case obj := <-out:
			fmt.Printf("\n\nss after but inside select: %+v", obj)
			statusRequest, ok := obj.(*velerov1api.ServerStatusRequest)

			// statusReq, ok := obj.(*velerov1api.ServerStatusRequest)
			if !ok {
				fmt.Printf("unexpected type %+v", obj)
				return nil, errors.New("unexpected type")
			}

			req = statusRequest
			// fmt.Println("\n\nCREATED NEW")

			break Loop
		case <-expired.C:
			fmt.Println("time out!!!!")
			return nil, errors.New("timed out waiting for server status request to be processed")
		}
	}

	return req, nil
}
