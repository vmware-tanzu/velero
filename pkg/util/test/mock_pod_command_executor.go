package test

import (
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"

	"github.com/heptio/ark/pkg/apis/ark/v1"
)

type MockPodCommandExecutor struct {
	mock.Mock
}

func (e *MockPodCommandExecutor) ExecutePodCommand(log logrus.FieldLogger, item map[string]interface{}, namespace, name, hookName string, hook *v1.ExecHook) error {
	args := e.Called(log, item, namespace, name, hookName, hook)
	return args.Error(0)
}
