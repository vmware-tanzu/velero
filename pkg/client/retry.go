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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateRetryGenerateName(client kbclient.Client, ctx context.Context, obj kbclient.Object) error {
	retryCreateFn := func() error {
		// needed to ensure that the name from the failed create isn't left on the object between retries
		obj.SetName("")
		return client.Create(ctx, obj, &kbclient.CreateOptions{})
	}
	if obj.GetGenerateName() != "" && obj.GetName() == "" {
		return retry.OnError(retry.DefaultRetry, apierrors.IsAlreadyExists, retryCreateFn)
	} else {
		return client.Create(ctx, obj, &kbclient.CreateOptions{})
	}
}

// two minute backoff
var twoMinBackoff = wait.Backoff{
	Factor:   5.0,
	Steps:    8,
	Duration: 10 * time.Millisecond,
	Jitter:   0.1,
	Cap:      2 * time.Minute,
}

// override backoff for testing
func TestOverrideBackoff() {
	twoMinBackoff = wait.Backoff{
		Factor:   1.0,
		Steps:    twoMinBackoff.Steps,
		Duration: 1,
		Jitter:   0.0,
		Cap:      2 * time.Minute,
	}
}

func TestResetBackoff() {
	twoMinBackoff = wait.Backoff{
		Factor:   5.0,
		Steps:    8,
		Duration: 10 * time.Millisecond,
		Jitter:   0.1,
		Cap:      2 * time.Minute,
	}
}

func GetBackoffSteps() int {
	return twoMinBackoff.Steps
}

// RetriesPhasePatchFunc accepts a patch function param, retrying when the provided retriable function returns true.
// We want retry to last up to 2 minutes: https://github.com/vmware-tanzu/velero/issues/7207#:~:text=A%20two%2Dminute%20retry%20is%20reasonable%20when%20there%20is%20API%20outage%20due%20to%20cert%20rotation.
func RetriesPhasePatchFunc(fn func() error, retriable func(error) bool) error {
	return retry.OnError(twoMinBackoff, func(err error) bool { return retriable(err) }, fn)
}

// RetriesPhasePatchFuncOnErrors accepts a patch function param, retrying when the error is not nil.
// We want retry to last up to 2 minutes: https://github.com/vmware-tanzu/velero/issues/7207#:~:text=A%20two%2Dminute%20retry%20is%20reasonable%20when%20there%20is%20API%20outage%20due%20to%20cert%20rotation.
func RetriesPhasePatchFuncOnErrors(fn func() error) error {
	return RetriesPhasePatchFunc(fn, func(err error) bool { return err != nil })
}
