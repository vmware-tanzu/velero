package k8s

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func InstallStorageClass(ctx context.Context, yaml string) error {
	fmt.Printf("Install storage class with %s.\n", yaml)
	err := KubectlApplyByFile(ctx, yaml)
	return err
}

func DeleteStorageClass(ctx context.Context, client TestClient, name string) error {
	if err := client.ClientGo.StorageV1().StorageClasses().Delete(ctx, name, v1.DeleteOptions{}); err != nil {
		return errors.Wrapf(err, "Could not retrieve storage classes %s", name)
	}
	return nil
}
