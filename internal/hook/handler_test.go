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

const (
	pod = `{
  "apiVersion": "v1",
  "kind": "Pod",
  "metadata": {
    "name": "nginx",
	"namespace": "nginx",
	"labels": {
	  "app": "nginx"
	}
  },
  "spec": {
    "containers": [
      {
        "name": "nginx",
        "image": "nginx:1.14.2",
        "ports": [
          {
            "containerPort": 80
          }
        ]
      }
    ]
  }
}
	`
)

func TestHandleResourceHooks(t *testing.T) {
	podvolumeBackupper := mock_podvolume.NewMockBackupper(t)
	podCMDExecutor := &test.MockPodCommandExecutor{}
	handler := NewHandler(podvolumeBackupper, podCMDExecutor)
	ctx := context.Background()
	log := logrus.New()
	res := &unstructured.Unstructured{}
	err := res.UnmarshalJSON([]byte(pod))
	require.NoError(t, err)
	var hooks []*ResourceHook

	// empty hooks list
	results := handler.HandleResourceHooks(ctx, log, res, hooks)
	require.Empty(t, results)

	// unknown hooks
	hooks = []*ResourceHook{
		{
			Name: "hook01",
			Type: "unknown",
		},
		{
			Name: "hook02",
			Type: "unknown",
		},
	}
	results = handler.HandleResourceHooks(ctx, log, res, hooks)
	require.Len(t, results, 2)
	assert.Equal(t, StatusFailed, results[0].Status)
	assert.Equal(t, StatusFailed, results[1].Status)

	// skip other hooks if the former one failed and marked as not continue
	podCMDExecutor.On("ExecutePodCommand", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(errors.New("failed to exec command"))
	hooks = []*ResourceHook{
		{
			Name:            "hook01",
			Type:            TypePodBackupPreHook,
			Spec:            &velerov1.ExecHook{},
			Resource:        res,
			ContinueOnError: true,
		},
		{
			Name:            "hook02",
			Type:            TypePodBackupPreHook,
			Spec:            &velerov1.ExecHook{},
			Resource:        res,
			ContinueOnError: false,
		},
		{
			Name:            "hook03",
			Type:            TypePodBackupPreHook,
			Spec:            &velerov1.ExecHook{},
			Resource:        res,
			ContinueOnError: false,
		},
	}
	results = handler.HandleResourceHooks(ctx, log, res, hooks)
	require.Len(t, results, 3)
	assert.Equal(t, StatusFailed, results[0].Status)
	assert.Equal(t, StatusFailed, results[1].Status)
	assert.Equal(t, StatusFailed, results[2].Status)

	// all completed
	podCMDExecutor.On("ExecutePodCommand").Unset()
	podCMDExecutor.On("ExecutePodCommand", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(nil)
	hooks = []*ResourceHook{
		{
			Name:            "hook01",
			Type:            TypePodBackupPreHook,
			Spec:            &velerov1.ExecHook{},
			Resource:        res,
			ContinueOnError: true,
		},
		{
			Name:            "hook02",
			Type:            TypePodBackupPreHook,
			Spec:            &velerov1.ExecHook{},
			Resource:        res,
			ContinueOnError: false,
		},
	}
	results = handler.HandleResourceHooks(ctx, log, res, hooks)
	require.Len(t, results, 2)
	assert.Equal(t, StatusCompleted, results[0].Status)
	assert.Equal(t, StatusCompleted, results[1].Status)
}

func TestAsyncHandleResourceHooksAndWaitAllResourceHooksCompleted(t *testing.T) {
	podvolumeBackupper := mock_podvolume.NewMockBackupper(t)
	podCMDExecutor := &test.MockPodCommandExecutor{}
	handler := NewHandler(podvolumeBackupper, podCMDExecutor)
	ctx := context.Background()
	log := logrus.New()
	res := &unstructured.Unstructured{}
	err := res.UnmarshalJSON([]byte(pod))
	require.NoError(t, err)

	podCMDExecutor.On("ExecutePodCommand", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(nil)
	hooks := []*ResourceHook{
		{
			Name:            "hook01",
			Type:            TypePodBackupPreHook,
			Spec:            &velerov1.ExecHook{},
			Resource:        res,
			ContinueOnError: true,
		},
		{
			Name:            "hook02",
			Type:            TypePodBackupPreHook,
			Spec:            &velerov1.ExecHook{},
			Resource:        res,
			ContinueOnError: false,
		},
	}
	handler.AsyncHandleResourceHooks(ctx, log, res, hooks)
	results := handler.WaitAllResourceHooksCompleted(ctx, log)
	require.NotNil(t, results)
	require.Equal(t, 2, results.Total)
	require.Equal(t, 2, results.Completed)
	assert.Equal(t, StatusCompleted, results.Results[0].Status)
	assert.Equal(t, StatusCompleted, results.Results[1].Status)
}

func Test_getResourceHookHandler(t *testing.T) {
	handler := &handler{}

	// pod backup pre hook
	resourceHookHandler, err := handler.getResourceHookHandler(TypePodBackupPreHook)
	require.NoError(t, err)
	assert.NotNil(t, resourceHookHandler)

	// pod backup post hook
	resourceHookHandler, err = handler.getResourceHookHandler(TypePodBackupPostHook)
	require.NoError(t, err)
	assert.NotNil(t, resourceHookHandler)

	// unknown hook
	_, err = handler.getResourceHookHandler("unknown")
	require.Error(t, err)
}
