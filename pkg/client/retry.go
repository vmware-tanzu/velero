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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
