package hook

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/vmware-tanzu/velero/pkg/podexec"
	"github.com/vmware-tanzu/velero/pkg/podvolume"
)

// Handler handles all the hooks of one backup or restore.
//
// The pod's backup post hooks cannot execute until the PVBs are processed, Handler leverages the podvolume.Backupper to check this,
// and the podvolume.Backupper is per backup/restore, so one instance of Handler can only handle the hooks for one backup or restore.
//
// Handler only handles the hooks of pods for now, but it can be extended
// to other resources or even the hook defined for backup/restore if needed
type Handler interface {
	// HandleResourceHooks handles a group of same type hooks for a specific resource, e.g. handles all backup pre hooks for one pod.
	// Because whether to execute the hook may depend on the execution result of previous hooks (e.g. hooks will not execute
	// if the previous hook is failed and marked as not continue), this function accepts a hook list as a group to handle.
	//
	// This function blocks until the hook completed, use "AsyncHandleResourceHooks()" instead if you want to handle the hooks asynchronously.
	//
	// The execution results are returned and also tracked inside the handler, calling the "WaitAllResourceHooksCompleted()" returns the results.
	//
	// This function only handles the hooks of pod for now, but it can be extended to other resources easily
	HandleResourceHooks(ctx context.Context, log logrus.FieldLogger, resource *unstructured.Unstructured, hooks []*ResourceHook) []*ResourceHookResult
	// AsyncHandleResourceHooks is the asynchronous version of "HandleResourceHooks()".
	//
	// Call "WaitAllHooksCompleted()" to wait all hooks completed and get the results.
	AsyncHandleResourceHooks(ctx context.Context, log logrus.FieldLogger, resource *unstructured.Unstructured, hooks []*ResourceHook)
	// WaitAllResourceHooksCompleted waits resource hooks completed and returns the execution results
	WaitAllResourceHooksCompleted(ctx context.Context, log logrus.FieldLogger) *ResourceHookResults
}

// make sure "handler" implements "Handler" interface
var _ Handler = &handler{}

func NewHandler(podVolumeBackupper podvolume.Backupper, podCommandExecutor podexec.PodCommandExecutor) Handler {
	return &handler{
		WaitGroup: &sync.WaitGroup{},
		results: &ResourceHookResults{
			RWMutex: &sync.RWMutex{},
			Results: []*ResourceHookResult{},
		},
		podVolumeBackupper: podVolumeBackupper,
		podCommandExecutor: podCommandExecutor,
	}
}

type handler struct {
	*sync.WaitGroup
	results            *ResourceHookResults
	podVolumeBackupper podvolume.Backupper
	podCommandExecutor podexec.PodCommandExecutor
}

func (h *handler) HandleResourceHooks(ctx context.Context, log logrus.FieldLogger, resource *unstructured.Unstructured, hooks []*ResourceHook) []*ResourceHookResult {
	if len(hooks) == 0 {
		return nil
	}

	var results []*ResourceHookResult
	// make sure the results are tracked inside the handler
	defer func() {
		for _, result := range results {
			h.results.AddResult(result)
		}
	}()

	markHooksFailed := func(hooks []*ResourceHook, err error) []*ResourceHookResult {
		now := time.Now()
		for _, hook := range hooks {
			results = append(results, &ResourceHookResult{
				Hook:      hook,
				Status:    StatusFailed,
				StartTime: now,
				EndTime:   now,
				Error:     err,
			})
		}
		return results
	}

	resourceHookHandler, err := h.getResourceHookHandler(hooks[0].Type)
	if err != nil {
		return markHooksFailed(hooks, errors.Wrapf(err, "failed to get the resource hook handler for type %q", hooks[0].Type))
	}

	if err = resourceHookHandler.WaitUntilReadyToExec(ctx, log, resource); err != nil {
		return markHooksFailed(hooks, errors.Wrap(err, "failed to wait ready to execute hook"))
	}

	for i, hook := range hooks {
		now := time.Now()
		result := &ResourceHookResult{
			Hook:      hook,
			StartTime: now,
		}

		// execution failed
		if err = resourceHookHandler.Exec(ctx, log, resource, hook); err != nil {
			result.Status = StatusFailed
			result.EndTime = time.Now()
			result.Error = err
			results = append(results, result)
			// skip
			if !hook.ContinueOnError {
				markHooksFailed(hooks[i+1:], errors.New("skip because the previous hook execution failed"))
				break
			}
			continue
		}

		// execution completed
		result.Status = StatusCompleted
		result.EndTime = time.Now()
		results = append(results, result)
	}

	return results
}

func (h *handler) AsyncHandleResourceHooks(ctx context.Context, log logrus.FieldLogger, resource *unstructured.Unstructured, hooks []*ResourceHook) {
	n := len(hooks)
	h.WaitGroup.Add(n)
	go func() {
		defer func() {
			for i := 0; i < n; i++ {
				h.Done() // decrements the WaitGroup counter by one
			}
		}()

		results := h.HandleResourceHooks(ctx, log, resource, hooks)
		_ = results
	}()
}

func (h *handler) WaitAllResourceHooksCompleted(ctx context.Context, log logrus.FieldLogger) *ResourceHookResults {
	h.Wait()
	return h.results
}

func (h *handler) getResourceHookHandler(hookType string) (ResourceHookHandler, error) {
	switch hookType {
	case TypePodBackupPreHook:
		return NewPodBackupPreHookHandler(h.podCommandExecutor), nil
	case TypePodBackupPostHook:
		return NewPodBackupPostHookHandler(h.podVolumeBackupper, h.podCommandExecutor), nil
	default:
		return nil, errors.Errorf("unknown hook type %q", hookType)
	}
}
