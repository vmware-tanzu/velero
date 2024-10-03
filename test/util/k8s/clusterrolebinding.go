/*
Copyright the Velero contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package k8s

import (
	"fmt"
	"os/exec"
	"strings"

	"golang.org/x/net/context"

	veleroexec "github.com/vmware-tanzu/velero/pkg/util/exec"
)

func KubectlDeleteClusterRoleBinding(ctx context.Context, name string) error {
	args := []string{"delete", "clusterrolebinding", name}
	fmt.Println(args)
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	fmt.Println(cmd)
	_, stderr, err := veleroexec.RunCommand(cmd)
	if strings.Contains(stderr, "NotFound") {
		fmt.Printf("Ignore error: %v\n", stderr)
		err = nil
	}
	return err
}

func KubectlCreateClusterRoleBinding(ctx context.Context, name, clusterrole, namespace, serviceaccount string) error {
	args := []string{"create", "clusterrolebinding", name, fmt.Sprintf("--clusterrole=%s", clusterrole), fmt.Sprintf("--serviceaccount=%s:%s", namespace, serviceaccount)}
	fmt.Println(args)
	return exec.CommandContext(ctx, "kubectl", args...).Run()
}
