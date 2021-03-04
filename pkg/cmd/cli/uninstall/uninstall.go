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

package uninstall

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"

	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/util/output"
	"github.com/vmware-tanzu/velero/pkg/install"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
)

// Uninstall uninstalls all components deployed using velero install command
func Uninstall(ctx context.Context, client *kubernetes.Clientset, extensionsClient *apiextensionsclientset.Clientset, veleroNamespace string) error {
	if veleroNamespace == "" {
		veleroNamespace = "velero"
	}
	err := DeleteNamespace(ctx, client, veleroNamespace)
	if err != nil {
		return errors.WithMessagef(err, "Uninstall failed removing Velero namespace %s", veleroNamespace)
	}

	rolebinding := install.ClusterRoleBinding(veleroNamespace)

	err = client.RbacV1().ClusterRoleBindings().Delete(ctx, rolebinding.Name, metav1.DeleteOptions{})
	if err != nil {
		return errors.WithMessagef(err, "Uninstall failed removing Velero cluster role binding %s", rolebinding)
	}
	veleroLabels := labels.FormatLabels(install.Labels())

	crds, err := extensionsClient.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{
		LabelSelector: veleroLabels,
	})
	if err != nil {
		return errors.WithMessagef(err, "Uninstall failed listing Velero crds")
	}
	for _, removeCRD := range crds.Items {
		err = extensionsClient.ApiextensionsV1().CustomResourceDefinitions().Delete(ctx, removeCRD.ObjectMeta.Name, metav1.DeleteOptions{})
		if err != nil {
			return errors.WithMessagef(err, "Uninstall failed removing CRD %s", removeCRD.ObjectMeta.Name)
		}
	}

	fmt.Println("Uninstalled Velero")
	return nil
}

func DeleteNamespace(ctx context.Context, client *kubernetes.Clientset, namespace string) error {
	err := client.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
	if err != nil {
		return errors.WithMessagef(err, "Delete namespace failed removing namespace %s", namespace)
	}
	return wait.Poll(1*time.Second, 3*time.Minute, func() (bool, error) {
		_, err := client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if err != nil {
			// Commented this out because not sure if removing this is okay
			// Printing this on Uninstall will lead to confusion
			// fmt.Printf("Namespaces.Get after delete return err %v\n", err)
			return true, nil // Assume any error means the delete was successful
		}
		return false, nil
	})
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
			veleroNs := strings.TrimSpace(f.Namespace())
			cl, extCl, err := kube.GetClusterClient()
			cmd.CheckError(err)
			cmd.CheckError(Uninstall(context.Background(), cl, extCl, veleroNs))
		},
	}

	output.BindFlags(c.Flags())
	output.ClearOutputFlagDefault(c)
	return c
}
