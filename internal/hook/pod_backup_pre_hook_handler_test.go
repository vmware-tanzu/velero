package hook

import (
	"context"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/test"
)

func TestWaitUntilReadyToExecOf_podBackupPreHookHandler(t *testing.T) {
	podCMDExecutor := &test.MockPodCommandExecutor{}
	handler := NewPodBackupPreHookHandler(podCMDExecutor)
	ctx := context.Background()
	log := logrus.New()
	res := &unstructured.Unstructured{}
	err := res.UnmarshalJSON([]byte(pod))
	require.NoError(t, err)

	err = handler.WaitUntilReadyToExec(ctx, log, res)
	assert.NoError(t, err)
}

func TestExecOf_podBackupPreHookHandler(t *testing.T) {
	podCMDExecutor := &test.MockPodCommandExecutor{}
	handler := NewPodBackupPreHookHandler(podCMDExecutor)
	ctx := context.Background()
	log := logrus.New()
	res := &unstructured.Unstructured{}
	err := res.UnmarshalJSON([]byte(pod))
	require.NoError(t, err)

	// not velerov1.ExecHook
	hook := &ResourceHook{
		Name: "hook01",
		Type: TypePodBackupPreHook,
		Spec: "invalid",
	}
	err = handler.Exec(ctx, log, res, hook)
	assert.Error(t, err)

	// command exec failed
	hook = &ResourceHook{
		Name: "hook01",
		Type: TypePodBackupPreHook,
		Spec: &velerov1.ExecHook{},
	}
	podCMDExecutor.On("ExecutePodCommand", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(errors.New("error"))
	err = handler.Exec(ctx, log, res, hook)
	assert.Error(t, err)

	// command exec succeed
	hook = &ResourceHook{
		Name: "hook01",
		Type: TypePodBackupPreHook,
		Spec: &velerov1.ExecHook{},
	}
	podCMDExecutor.On("ExecutePodCommand").Unset()
	podCMDExecutor.On("ExecutePodCommand", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(nil)
	err = handler.Exec(ctx, log, res, hook)
	assert.NoError(t, err)
}
