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
	"math"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateRetryGenerateName(ctx context.Context, client kbclient.Client, obj kbclient.Object) error {
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

// CapBackoff provides a backoff with a set backoff cap
func CapBackoff(cap time.Duration) wait.Backoff {
	if cap < 0 {
		cap = 0
	}
	return wait.Backoff{
		Steps:    math.MaxInt,
		Duration: 10 * time.Millisecond,
		Cap:      cap,
		Factor:   retry.DefaultBackoff.Factor,
		Jitter:   retry.DefaultBackoff.Jitter,
	}
}

// RetryOnRetriableMaxBackOff accepts a patch function param, retrying when the provided retriable function returns true.
func RetryOnRetriableMaxBackOff(maxDuration time.Duration, fn func() error, retriable func(error) bool) error {
	return retry.OnError(CapBackoff(maxDuration), func(err error) bool { return retriable(err) }, fn)
}

// RetryOnErrorMaxBackOff accepts a patch function param, retrying when the error is not nil.
func RetryOnErrorMaxBackOff(maxDuration time.Duration, fn func() error) error {
	return RetryOnRetriableMaxBackOff(maxDuration, fn, func(err error) bool { return err != nil })
}
