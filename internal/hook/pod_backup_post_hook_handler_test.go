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
	mock_podvolume "github.com/vmware-tanzu/velero/pkg/podvolume/mocks"
	"github.com/vmware-tanzu/velero/pkg/test"
)

func TestWaitUntilReadyToExecOf_podBackupPostHookHandler(t *testing.T) {
	podvolumeBackupper := mock_podvolume.NewMockBackupper(t)
	podCMDExecutor := &test.MockPodCommandExecutor{}
	handler := NewPodBackupPostHookHandler(podvolumeBackupper, podCMDExecutor)
	ctx := context.Background()
	log := logrus.New()
	res := &unstructured.Unstructured{}
	err := res.UnmarshalJSON([]byte(pod))
	require.NoError(t, err)

	// failed to wait all pod volumes processed
	podvolumeBackupper.On("WaitAllPodVolumesProcessed", mock.Anything).Return(nil, errors.New("timeout"))
	err = handler.WaitUntilReadyToExec(ctx, log, res)
	assert.Error(t, err)

	// succeed to wait all pod volumes processed
	podvolumeBackupper.On("WaitAllPodVolumesProcessed").Unset()
	podvolumeBackupper.On("WaitAllPodVolumesProcessed", mock.Anything).Return(nil, nil)
	err = handler.WaitUntilReadyToExec(ctx, log, res)
	assert.NoError(t, err)
}

func TestExecOf_podBackupPostHookHandler(t *testing.T) {
	podvolumeBackupper := mock_podvolume.NewMockBackupper(t)
	podCMDExecutor := &test.MockPodCommandExecutor{}
	handler := NewPodBackupPostHookHandler(podvolumeBackupper, podCMDExecutor)
	ctx := context.Background()
	log := logrus.New()
	res := &unstructured.Unstructured{}
	err := res.UnmarshalJSON([]byte(pod))
	require.NoError(t, err)

	// not velerov1.ExecHook
	hook := &ResourceHook{
		Name: "hook01",
		Type: TypePodBackupPostHook,
		Spec: "invalid",
	}
	err = handler.Exec(ctx, log, res, hook)
	assert.Error(t, err)

	// command exec failed
	hook = &ResourceHook{
		Name: "hook01",
		Type: TypePodBackupPostHook,
		Spec: &velerov1.ExecHook{},
	}
	podCMDExecutor.On("ExecutePodCommand", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(errors.New("error"))
	err = handler.Exec(ctx, log, res, hook)
	assert.Error(t, err)

	// command exec succeed
	hook = &ResourceHook{
		Name: "hook01",
		Type: TypePodBackupPostHook,
		Spec: &velerov1.ExecHook{},
	}
	podCMDExecutor.On("ExecutePodCommand").Unset()
	podCMDExecutor.On("ExecutePodCommand", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(nil)
	err = handler.Exec(ctx, log, res, hook)
	assert.NoError(t, err)
}
