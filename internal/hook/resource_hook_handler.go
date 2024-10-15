package hook

import (
	"context"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ResourceHookHandler executes one hook for a specific Kubernetes resource
type ResourceHookHandler interface {
	// WaitUntilReadyToExec waits the provided resource to be ready for hooks to execute
	WaitUntilReadyToExec(ctx context.Context, log logrus.FieldLogger, resource *unstructured.Unstructured) error
	// Exec executes the hook
	Exec(ctx context.Context, log logrus.FieldLogger, resource *unstructured.Unstructured, hook *ResourceHook) error
}
