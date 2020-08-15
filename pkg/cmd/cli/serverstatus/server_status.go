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
	"sync"

	"github.com/pkg/errors"
	"k8s.io/client-go/tools/cache"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
)

func GetServerStatus(mgr manager.Manager, namespace string, ctx context.Context) (*velerov1api.ServerStatusRequest, error) {
	serverReq := builder.ForServerStatusRequest(namespace, "", "0").ObjectMeta(builder.WithGenerateName("velero-cli-")).Result()

	informer, err := mgr.GetCache().GetInformer(ctx, serverReq)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	addFuncResult := make(chan interface{}, 1)
	addFunc := func(result interface{}) {
		addFuncResult <- result
	}
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: addFunc,
	})

	stopMgr := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer func() {
			wg.Done()
		}()
		if err := mgr.Start(stopMgr); err != nil {
			addFuncResult <- errors.New("manager didn't start")
		}
	}()

	// Terminate the manager goroutine and wait for it to respond
	// that it has terminated.
	defer func() {
		close(stopMgr)
		wg.Wait()
	}()

	if err := mgr.GetClient().Create(ctx, serverReq, &kbclient.CreateOptions{}); err != nil {
		return nil, errors.WithStack(err)
	}

	var result interface{}
	select {
	case result = <-addFuncResult:
	case <-ctx.Done():
	}

	if result == nil {
		return nil, errors.New("timed out")
	}

	switch req := result.(type) {
	case *velerov1api.ServerStatusRequest:
		// if req.Status.Phase != velerov1api.ServerStatusRequestPhaseProcessed {
		// 	return nil, errors.New("request not processed")
		// }
		return req, nil
	case error:
		return nil, err
	default:
		return nil, errors.New("unknown response")
	}
}
