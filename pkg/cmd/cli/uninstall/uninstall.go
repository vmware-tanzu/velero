/*
Copyright 2020 the Velero contributors.

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

package uninstall

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/util/output"
	"github.com/vmware-tanzu/velero/pkg/install"
)

func Uninstall(c *cobra.Command, f client.Factory) error {
	resources := install.AllCRDs()

	dynamicClient, err := f.DynamicClient()
	if err != nil {
		return err
	}

	factory := client.NewDynamicFactory(dynamicClient)

	veleroNs := strings.TrimSpace(f.Namespace())

	if veleroNs == "" {
		veleroNs = "velero"
	}

	ds := install.DaemonSet(veleroNs, []install.PodTemplateOption{}...)
	if err := install.AppendUnstructured(resources, ds); err != nil {
		return err
	}

	de := install.Deployment(veleroNs, []install.PodTemplateOption{}...)
	if err := install.AppendUnstructured(resources, de); err != nil {
		return err
	}

	se := install.Secret(veleroNs, nil)
	if err := install.AppendUnstructured(resources, se); err != nil {
		return err
	}

	sa := install.ServiceAccount(veleroNs, map[string]string{})
	if err := install.AppendUnstructured(resources, sa); err != nil {
		return err
	}

	crb := install.ClusterRoleBinding(veleroNs)
	if err := install.AppendUnstructured(resources, crb); err != nil {
		return err
	}

	ns := install.Namespace(veleroNs)
	if err := install.AppendUnstructured(resources, ns); err != nil {
		return err
	}

	for _, r := range resources.Items {
		cl, err := install.CreateClient(&r, factory, os.Stdout)
		if err != nil {
			return err
		}
		fmt.Printf("%s/%s: attempting to delete resource\n", r.GetKind(), r.GetName())
		if e := cl.Delete(r.GetName(), metav1.DeleteOptions{}); e != nil {
			if apierrors.IsNotFound(e) {
				fmt.Printf("%s/%s: skipping because resource is not present in the cluster\n", r.GetKind(), r.GetName())
				continue
			}
		}
		fmt.Printf("%s/%s: deleted resource\n", r.GetKind(), r.GetName())
	}

	return nil
}

// NewCommand creates a cobra command.
func NewCommand(f client.Factory) *cobra.Command {
	c := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall Velero",
		Long: `Uninstall Velero along with the CRDs.

The '--namespace' flag can be used to specify the namespace where velero is installed (default: velero).
		`,
		Example: `# velero uninstall -n backup`,
		Run: func(c *cobra.Command, args []string) {
			cmd.CheckError(Uninstall(c, f))
		},
	}

	output.BindFlags(c.Flags())
	output.ClearOutputFlagDefault(c)
	return c
}
