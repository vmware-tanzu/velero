package k8s

import (
	"context"
	"fmt"
)

func InstallStorageClass(ctx context.Context, yaml string) error {
	fmt.Printf("Install storage class with %s.\n", yaml)
	err := KubectlApplyByFile(ctx, yaml)
	return err
}
