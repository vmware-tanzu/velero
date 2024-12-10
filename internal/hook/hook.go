package hook

import (
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	TypePodBackupPreHook   string = "POD_BACKUP_PRE_HOOk"
	TypePodBackupPostHook  string = "POD_BACKUP_POST_HOOk"
	TypePodRestoreInitHook string = "POD_RESTORE_INIT_HOOk"
	TypePodRestoreExecHook string = "POD_RESTORE_EXEC_HOOk"

	StatusCompleted string = "COMPLETED" // hook execution completed
	StatusFailed    string = "FAILED"    // hook execution failed
)

// ResourceHook is the general resource hook definition handled by the Handler
type ResourceHook struct {
	Name string
	Type string
	// multiple hooks may be defined under the same name backup/restore hook spec, the index here
	// indicate the location of the hook in the list. This is helpful to locate the hook when needed
	Index           int
	Spec            interface{}
	ContinueOnError bool
	Resource        *unstructured.Unstructured
}

// the execution result for a specific resource hook
type ResourceHookResult struct {
	Hook      *ResourceHook
	Status    string
	StartTime time.Time
	EndTime   time.Time
	Error     error
}

// ResourceHookResults hold the execution results for all resource hooks of a specific backup/restore
type ResourceHookResults struct {
	*sync.RWMutex
	Total     int
	Completed int
	Failed    int
	Results   []*ResourceHookResult
}

func (r *ResourceHookResults) AddResult(result *ResourceHookResult) {
	r.Lock()
	defer r.Unlock()

	r.Total++
	switch result.Status {
	case StatusCompleted:
		r.Completed++
	case StatusFailed:
		r.Failed++
	}

	r.Results = append(r.Results, result)
}
