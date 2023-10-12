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

package client

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// MinuteBackoff is a retry.DefaultBackoff that retries for at least a minute (60000ms) but no more than 2 minutes (120000ms).
var MinuteBackoff = func() wait.Backoff {
	mb := retry.DefaultBackoff
	// TotalDuration = 0ms + 10ms + 50ms + 250ms + 1250ms + 6250ms + 31250ms + 60000ms = 99,060 ms > 1 minute
	// 7 steps
	mb.Steps = 7
	mb.Cap = time.Minute
	return mb
}()

func CreateRetryGenerateName(client kbclient.Client, ctx context.Context, obj kbclient.Object) error {
	return CreateRetryGenerateNameWithFunc(obj, func() error {
		return client.Create(ctx, obj, &kbclient.CreateOptions{})
	})
}

func CreateRetryGenerateNameWithFunc(obj kbclient.Object, createFn func() error) error {
	retryCreateFn := func() error {
		// needed to ensure that the name from the failed create isn't left on the object between retries
		obj.SetName("")
		return createFn()
	}
	if obj.GetGenerateName() != "" && obj.GetName() == "" {
		return retry.OnError(retry.DefaultRetry, apierrors.IsAlreadyExists, retryCreateFn)
	} else {
		return createFn()
	}
}

type GetFunc func(name string) (*unstructured.Unstructured, error)

func GetFuncForCacheLister(lister cache.GenericNamespaceLister) GetFunc {
	return func(name string) (*unstructured.Unstructured, error) {
		runtimeObj, err := lister.Get(name)
		if err != nil {
			return nil, err
		}
		u, ok := runtimeObj.(*unstructured.Unstructured)
		if !ok {
			return nil, fmt.Errorf("expected *unstructured.Unstructured but got %T", u)
		}
		return u, nil
	}
}

func GetFuncForDynamicClient(client Dynamic, getOptions metav1.GetOptions) GetFunc {
	return func(name string) (*unstructured.Unstructured, error) {
		return client.Get(name, getOptions)
	}
}

func GetRetriable(getFuncIn GetFunc, name string, retriable func(error) bool) (*unstructured.Unstructured, error) {
	var clusterObj *unstructured.Unstructured
	getFunc := func() error {
		var err error
		clusterObj, err = getFuncIn(name)
		return err
	}
	err := retry.OnError(MinuteBackoff, retriable, getFunc)
	return clusterObj, err
}
