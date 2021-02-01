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
	// Pass the provided Context down to the run function.
	f := func(ctx context.Context) error {
		return p.Run(ctx, numWorkers)
	}
	return manager.RunnableFunc(f)
}
