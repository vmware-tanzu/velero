package hook

import (
	"context"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/podexec"
	"github.com/vmware-tanzu/velero/pkg/podvolume"
)

var _ ResourceHookHandler = &podBackupPostHookHandler{}

func NewPodBackupPostHookHandler(backupper podvolume.Backupper, executor podexec.PodCommandExecutor) ResourceHookHandler {
	return &podBackupPostHookHandler{
		podVolumeBackupper: backupper,
		podCommandExecutor: executor,
	}
}

// podBackupPostHookHandler handles the post backup hooks for pods
type podBackupPostHookHandler struct {
	podVolumeBackupper podvolume.Backupper
	podCommandExecutor podexec.PodCommandExecutor
}

// Don't need to wait when using CSI snapshot, because:
//
//	PVCs are backed up before Pods (Pod's BIA returns PVCs as additional resources).
//	PVC's BIA creates VS and returns it as additional resource.
//	VS's BIA waits VSC's handle ready: https://github.com/vmware-tanzu/velero/blob/v1.14.1/pkg/backup/actions/csi/volumesnapshot_action.go#L118
//
// So when Pods are backed up (when hooks execute), the snapshot is taken already
func (p *podBackupPostHookHandler) WaitUntilReadyToExec(ctx context.Context, log logrus.FieldLogger, res *unstructured.Unstructured) error {
	// wait all PVBs processed
	_, err := p.podVolumeBackupper.WaitAllPodVolumesProcessed(log)
	return err
}

func (p *podBackupPostHookHandler) Exec(ctx context.Context, log logrus.FieldLogger, res *unstructured.Unstructured, hook *ResourceHook) error {
	execHook, ok := hook.Spec.(*velerov1.ExecHook)
	if !ok {
		return errors.New("failed to convert to ExecHook")
	}
	pod := &corev1.Pod{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(res.UnstructuredContent(), pod); err != nil {
		return errors.Wrap(err, "failed to convert Unstructured to pod")
	}
	if err := p.podCommandExecutor.ExecutePodCommand(log, res.UnstructuredContent(), pod.Namespace, pod.Name, hook.Name, execHook); err != nil {
		return errors.Wrap(err, "failed to execute pod command")
	}
	return nil
}
