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

package hook

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/podexec"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
)

type WaitExecHookHandler interface {
	HandleHooks(
		ctx context.Context,
		log logrus.FieldLogger,
		pod *v1.Pod,
		byContainer map[string][]PodExecRestoreHook,
	) []error
}

type ListWatchFactory interface {
	NewListWatch(namespace string, selector fields.Selector) cache.ListerWatcher
}

type DefaultListWatchFactory struct {
	PodsGetter cache.Getter
}

func (d *DefaultListWatchFactory) NewListWatch(namespace string, selector fields.Selector) cache.ListerWatcher {
	return cache.NewListWatchFromClient(d.PodsGetter, "pods", namespace, selector)
}

var _ ListWatchFactory = &DefaultListWatchFactory{}

type DefaultWaitExecHookHandler struct {
	ListWatchFactory   ListWatchFactory
	PodCommandExecutor podexec.PodCommandExecutor
}

var _ WaitExecHookHandler = &DefaultWaitExecHookHandler{}

func (e *DefaultWaitExecHookHandler) HandleHooks(
	ctx context.Context,
	log logrus.FieldLogger,
	pod *v1.Pod,
	byContainer map[string][]PodExecRestoreHook,
) []error {
	if pod == nil {
		return nil
	}

	// If hooks are defined for a container that does not exist in the pod log a warning and discard
	// those hooks to avoid waiting for a container that will never become ready. After that if
	// there are no hooks left to be executed return immediately.
	for containerName := range byContainer {
		if !podHasContainer(pod, containerName) {
			log.Warningf("Pod %s does not have container %s: discarding post-restore exec hooks", kube.NamespaceAndName(pod), containerName)
			delete(byContainer, containerName)
		}
	}
	if len(byContainer) == 0 {
		return nil
	}

	// Every hook in every container can have its own wait timeout. Rather than setting up separate
	// contexts for each, find the largest wait timeout for any hook that should be executed in
	// the pod and watch the pod for up to that long. Before executing any hook in a container,
	// check if that hook has a timeout and skip execution if expired.
	ctx, cancel := context.WithCancel(ctx)
	maxWait := maxHookWait(byContainer)
	// If no hook has a wait timeout then this function will continue waiting for containers to
	// become ready until the shared hook context is canceled.
	if maxWait > 0 {
		ctx, cancel = context.WithTimeout(ctx, maxWait)
	}
	waitStart := time.Now()

	var errors []error

	// The first time this handler is called after a container starts running it will execute all
	// pending hooks for that container. Subsequent invocations of this handler will never execute
	// hooks in that container. It uses the byContainer map to keep track of which containers have
	// not yet been observed to be running. It relies on the Informer not to be called concurrently.
	// When a container is observed running and its hooks are executed, the container is deleted
	// from the byContainer map. When the map is empty the watch is ended.
	handler := func(newObj interface{}) {
		newPod, ok := newObj.(*v1.Pod)
		if !ok {
			return
		}

		podLog := log.WithFields(
			logrus.Fields{
				"pod": kube.NamespaceAndName(newPod),
			},
		)

		if newPod.Status.Phase == v1.PodSucceeded || newPod.Status.Phase == v1.PodFailed {
			err := fmt.Errorf("Pod entered phase %s before some post-restore exec hooks ran", newPod.Status.Phase)
			podLog.Warning(err)
			cancel()
			return
		}

		for containerName, hooks := range byContainer {
			if !isContainerRunning(newPod, containerName) {
				podLog.Infof("Container %s is not running: post-restore hooks will not yet be executed", containerName)
				continue
			}
			podMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(newPod)
			if err != nil {
				podLog.WithError(err).Error("error unstructuring pod")
				cancel()
				return
			}

			// Sequentially run all hooks for the ready container. The container's hooks are not
			// removed from the byContainer map until all have completed so that if one fails
			// remaining unexecuted hooks can be handled by the outer function.
			for i, hook := range hooks {
				// This indicates to the outer function not to handle this hook as unexecuted in
				// case of terminating before deleting this container's slice of hooks from the
				// byContainer map.
				byContainer[containerName][i].executed = true

				hookLog := podLog.WithFields(
					logrus.Fields{
						"hookSource": hook.HookSource,
						"hookType":   "exec",
						"hookPhase":  "post",
					},
				)
				// Check the individual hook's wait timeout is not expired
				if hook.Hook.WaitTimeout.Duration != 0 && time.Since(waitStart) > hook.Hook.WaitTimeout.Duration {
					err := fmt.Errorf("Hook %s in container %s expired before executing", hook.HookName, hook.Hook.Container)
					hookLog.Error(err)
					if hook.Hook.OnError == velerov1api.HookErrorModeFail {
						errors = append(errors, err)
						cancel()
						return
					}
				}
				eh := &velerov1api.ExecHook{
					Container: hook.Hook.Container,
					Command:   hook.Hook.Command,
					OnError:   hook.Hook.OnError,
					Timeout:   hook.Hook.ExecTimeout,
				}
				if err := e.PodCommandExecutor.ExecutePodCommand(hookLog, podMap, pod.Namespace, pod.Name, hook.HookName, eh); err != nil {
					hookLog.WithError(err).Error("Error executing hook")
					if hook.Hook.OnError == velerov1api.HookErrorModeFail {
						errors = append(errors, err)
						cancel()
						return
					}
				}
			}
			delete(byContainer, containerName)
		}
		if len(byContainer) == 0 {
			cancel()
		}
	}

	selector := fields.OneTermEqualSelector("metadata.name", pod.Name)
	lw := e.ListWatchFactory.NewListWatch(pod.Namespace, selector)

	_, podWatcher := cache.NewInformer(lw, pod, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: handler,
		UpdateFunc: func(_, newObj interface{}) {
			handler(newObj)
		},
		DeleteFunc: func(obj interface{}) {
			err := fmt.Errorf("Pod %s deleted before all hooks were executed", kube.NamespaceAndName(pod))
			log.Error(err)
			cancel()
		},
	})

	podWatcher.Run(ctx.Done())

	// There are some cases where this function could return with unexecuted hooks: the pod may
	// be deleted, a hook with OnError mode Fail could fail, or it may timeout waiting for
	// containers to become ready.
	// Each unexecuted hook is logged as an error but only hooks with OnError mode Fail return
	// an error from this function.
	for _, hooks := range byContainer {
		for _, hook := range hooks {
			if hook.executed {
				continue
			}
			err := fmt.Errorf("Hook %s in container %s in pod %s not executed: %v", hook.HookName, hook.Hook.Container, kube.NamespaceAndName(pod), ctx.Err())
			hookLog := log.WithFields(
				logrus.Fields{
					"hookSource": hook.HookSource,
					"hookType":   "exec",
					"hookPhase":  "post",
				},
			)
			hookLog.Error(err)
			if hook.Hook.OnError == velerov1api.HookErrorModeFail {
				errors = append(errors, err)
			}
		}
	}

	return errors
}

func podHasContainer(pod *v1.Pod, containerName string) bool {
	if pod == nil {
		return false
	}
	for _, c := range pod.Spec.Containers {
		if c.Name == containerName {
			return true
		}
	}

	return false
}

func isContainerRunning(pod *v1.Pod, containerName string) bool {
	if pod == nil {
		return false
	}
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name != containerName {
			continue
		}
		return cs.State.Running != nil
	}

	return false
}

// maxHookWait returns 0 to mean wait indefinitely. Any hook without a wait timeout will cause this
// function to return 0.
func maxHookWait(byContainer map[string][]PodExecRestoreHook) time.Duration {
	var maxWait time.Duration
	for _, hooks := range byContainer {
		for _, hook := range hooks {
			if hook.Hook.WaitTimeout.Duration <= 0 {
				return 0
			}
			if hook.Hook.WaitTimeout.Duration > maxWait {
				maxWait = hook.Hook.WaitTimeout.Duration
			}
		}
	}
	return maxWait
}
