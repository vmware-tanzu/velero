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
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"

	// apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeerrs "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/cli"
	"github.com/vmware-tanzu/velero/pkg/cmd/util/output"
	"github.com/vmware-tanzu/velero/pkg/install"
)

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

			if !cli.GetConfirmation() {
				// Don't do anything unless we get confirmation
				return
			}

			kbClient, err := f.KubebuilderClient()
			cmd.CheckError(err)
			cmd.CheckError(Run(kbClient, f.Namespace()))
		},
	}

	output.BindFlags(c.Flags())
	output.ClearOutputFlagDefault(c)
	return c
}

// Run removes all components that were deployed using the Velero install command
func Run(kbClient kbclient.Client, namespace string) error {
	var errs []error

	// namespace
	ns := &corev1.Namespace{}
	key := kbclient.ObjectKey{Name: namespace}
	if err := kbClient.Get(context.Background(), key, ns); err != nil {
		if apierrors.IsNotFound(err) {
			fmt.Printf("Velero installation namespace %q does not exist, skipping.\n", namespace)
		} else {
			errs = append(errs, errors.WithStack(err))
		}
	} else {
		if ns.Status.Phase == corev1.NamespaceTerminating {
			fmt.Printf("Velero installation namespace %q is terminating.\n", namespace)
		} else {
			if err := kbClient.Delete(context.Background(), ns); err != nil {
				errs = append(errs, errors.WithStack(err))
			}
		}
	}

	time.Sleep(time.Second * 60)

	// rolebinding
	crb := install.ClusterRoleBinding(namespace)
	key = kbclient.ObjectKey{Name: crb.Name, Namespace: namespace}
	if err := kbClient.Get(context.Background(), key, crb); err != nil {
		if apierrors.IsNotFound(err) {
			fmt.Printf("Velero installation rolebinding %q does not exist, skipping.\n", crb.Name)
		} else {
			errs = append(errs, errors.WithStack(err))
		}
	} else {
		if err := kbClient.Delete(context.Background(), crb); err != nil {
			errs = append(errs, errors.WithStack(err))
		}
	}

	// CRDs
	veleroLabels := labels.FormatLabels(install.Labels())
	crdList := apiextv1beta1.CustomResourceDefinitionList{}
	opts := kbclient.ListOptions{
		Namespace: namespace,
		Raw: &metav1.ListOptions{
			LabelSelector: veleroLabels,
		},
	}
	if err := kbClient.List(context.Background(), &crdList, &opts); err != nil {
		errs = append(errs, errors.WithStack(err))
	} else {
		if len(crdList.Items) == 0 {
			fmt.Print("Velero CRDs do not exist, skipping.\n")
		} else {
			for _, crd := range crdList.Items {
				if err := kbClient.Delete(context.Background(), &crd); err != nil {
					errs = append(errs, errors.WithStack(err))
				}
			}
		}
	}

	if kubeerrs.NewAggregate(errs) != nil {
		fmt.Printf("Errors while attempting to uninstall Velero: %q", kubeerrs.NewAggregate(errs))
		return kubeerrs.NewAggregate(errs)
	}

	fmt.Println("Velero uninstalled ⛵")
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
