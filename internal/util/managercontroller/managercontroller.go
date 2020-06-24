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

// TODO(2.0) After converting all controllers to runttime-controller,
// the functions in this file will no longer be needed and should be removed.
package managercontroller

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/vmware-tanzu/velero/pkg/controller"
)

// Runnable will turn a "regular" runnable component (such as a controller)
// into a controller-runtime Runnable
func Runnable(p controller.Interface, numWorkers int) manager.Runnable {
	f := func(stop <-chan struct{}) error {

		// Create a cancel context for handling the stop signal.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// If a signal is received on the stop channel, cancel the
		// context. This will propagate the cancel into the p.Run
		// function below.
		go func() {
			select {
			case <-stop:
				cancel()
			case <-ctx.Done():
			}
		}()

		// This is a blocking call that either completes
		// or is cancellable on receiving a stop signal.
		return p.Run(ctx, numWorkers)
	}
	return manager.RunnableFunc(f)
}
