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
)

var _ ResourceHookHandler = &podBackupPreHookHandler{}

func NewPodBackupPreHookHandler(executor podexec.PodCommandExecutor) ResourceHookHandler {
	return &podBackupPreHookHandler{
		podCommandExecutor: executor,
	}
}

// podBackupPreHookHandler handles pre backup hooks for pods
type podBackupPreHookHandler struct {
	podCommandExecutor podexec.PodCommandExecutor
}

func (p *podBackupPreHookHandler) WaitUntilReadyToExec(ctx context.Context, log logrus.FieldLogger, res *unstructured.Unstructured) error {
	// no-op
	return nil
}

func (p *podBackupPreHookHandler) Exec(ctx context.Context, log logrus.FieldLogger, res *unstructured.Unstructured, hook *ResourceHook) error {
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
