package hook

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/podexec"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

// WaitExecHookHandler executes hooks in a pod's containers once they become ready.
type WaitExecHookHandler interface {
	HandleHooks(
		ctx context.Context,
		log logrus.FieldLogger,
		pod *v1.Pod,
		byContainer map[string][]NamedExecRestoreHook,
	) []error
}

type DefaultWaitExecHookHandler struct {
	PodsGetter         cache.Getter
	PodCommandExecutor podexec.PodCommandExecutor
}

var _ WaitExecHookHandler = &DefaultWaitExecHookHandler{}

func (e *DefaultWaitExecHookHandler) HandleHooks(
	ctx context.Context,
	log logrus.FieldLogger,
	pod *v1.Pod,
	byContainer map[string][]NamedExecRestoreHook,
) []error {
	// If hooks are defined for a container that does not exist in the pod log a warning and discard
	// those hooks to avoid waiting for a container that will never become ready. After that if
	// there are no hooks left to be executed return immediately.
	for containerName := range byContainer {
		if !podHasContainer(pod, containerName) {
			log.Warningf("Pod %s/%s does not have container %s: discarding post-restore exec hooks", pod.Namespace, pod.Name, containerName)
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
	var maxWait time.Duration
	for _, hooks := range byContainer {
		for _, hook := range hooks {
			if hook.Hook.WaitTimeout.Duration > maxWait {
				maxWait = hook.Hook.WaitTimeout.Duration
			}
		}
	}
	if maxWait > 0 {
		ctx, cancel = context.WithTimeout(ctx, maxWait)
	}
	waitStart := time.Now()

	var errors []error

	selector := fields.OneTermEqualSelector("metadata.name", pod.Name)
	lw := cache.NewListWatchFromClient(e.PodsGetter, "pods", pod.Namespace, selector)
	_, controller := cache.NewInformer(lw, pod, 0, cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(_, newObj interface{}) {
			newPod := newObj.(*v1.Pod)

			if newPod.Status.Phase == v1.PodSucceeded || newPod.Status.Phase == v1.PodFailed {
				err := fmt.Errorf("Pod %s/%s entered phase %s before some post-restore exec hooks ran", newPod.Namespace, newPod.Name, newPod.Status.Phase)
				log.Warning(err)
				cancel()
				return
			}

			for containerName, hooks := range byContainer {
				if !isContainerReady(newPod, containerName) {
					continue
				}
				podMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(newPod)
				if err != nil {
					log.WithError(err).Errorf("error unstructuring pod %s/%s", newPod.Namespace, newPod.Name)
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

					hookLog := log.WithFields(
						logrus.Fields{
							"hookSource": hook.Source,
							"hookType":   "exec",
							"hookPhase":  "post",
						},
					)
					// Check the individual hook's wait timeout is not expired
					if hook.Hook.WaitTimeout.Duration != 0 && time.Since(waitStart) > hook.Hook.WaitTimeout.Duration {
						err := fmt.Errorf("Hook %s in container %s in pod %s/%s expired before executing", hook.Name, hook.Hook.Container, pod.Namespace, pod.Name)
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
					if err := e.PodCommandExecutor.ExecutePodCommand(hookLog, podMap, pod.Namespace, pod.Name, hook.Name, eh); err != nil {
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
		},
		DeleteFunc: func(obj interface{}) {
			err := fmt.Errorf("Pod %s/%s deleted before all hooks were executed", pod.Namespace, pod.Name)
			log.Error(err)
			cancel()
		},
	})

	controller.Run(ctx.Done())

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
			err := fmt.Errorf("Hook %s in container %s in pod %s/%s not executed", hook.Hook.Container, pod.Namespace, pod.Name)
			hookLog := log.WithFields(
				logrus.Fields{
					"hookSource": hook.Source,
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

func isContainerReady(pod *v1.Pod, containerName string) bool {
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
