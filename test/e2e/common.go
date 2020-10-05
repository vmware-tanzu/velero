package e2e

import (
	"os/exec"

	"golang.org/x/net/context"
)

func CreateNamespace(ctx context.Context, namespace string) error {
	// TODO - should we talk directly to the API server?
	err := exec.CommandContext(ctx, "kubectl", "create", "namespace", namespace).Run()
	return err
}

func RemoveNamespace(ctx context.Context, namespace string) error {
	// TODO - should we talk directly to the API server?
	err := exec.CommandContext(ctx, "kubectl", "delete", "namespace", namespace).Run()
	return err
}

func NamespaceExists(ctx context.Context, namespace string) (bool, error) {
	return false, nil
}
