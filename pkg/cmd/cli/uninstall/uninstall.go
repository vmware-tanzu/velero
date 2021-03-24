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
	"github.com/spf13/pflag"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeerrs "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"

	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/cli"
	"github.com/vmware-tanzu/velero/pkg/install"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
)

// uninstallOptions collects all the options for uninstalling Velero from a Kubernetes cluster.
type uninstallOptions struct {
	wait, force bool
}

// BindFlags adds command line values to the options struct.
func (o *uninstallOptions) BindFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&o.wait, "wait", o.wait, "Wait for Velero deployment to be ready. Optional.")
	flags.BoolVar(&o.force, "force", o.force, "Forces the Velero uninstall. Optional.")
}

// NewCommand creates a cobra command.
func NewCommand(f client.Factory) *cobra.Command {
	o := &uninstallOptions{}

	c := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall Velero",
		Long: `Uninstall Velero along with the CRDs.

The '--namespace' flag can be used to specify the namespace where velero is installed (default: velero).
Use '--wait' to wait for the Velero Deployment to be ready before proceeding.
Use '--force' to skip the prompt confirming if you want to uninstall Velero.
		`,
		Example: `# velero uninstall -n staging`,
		Run: func(c *cobra.Command, args []string) {

			// Confirm if not asked to force-skip confirmation
			if !o.force {
				fmt.Println("You are about to uninstall Velero.")
				if !cli.GetConfirmation() {
					// Don't do anything unless we get confirmation
					return
				}
			}

			client, extCl, err := kube.GetClusterClient()
			cmd.CheckError(err)
			cmd.CheckError(Run(context.Background(), client, extCl, f.Namespace(), o.wait))
		},
	}

	o.BindFlags(c.Flags())
	return c
}

// Run removes all components that were deployed using the Velero install command
func Run(ctx context.Context, client *kubernetes.Clientset, extensionsClient *apiextensionsclientset.Clientset, namespace string, waitToTerminate bool) error {
	var errs []error

	// namespace
	ns, err := client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			fmt.Printf("Velero installation namespace %q does not exist, skipping.\n", namespace)
		} else {
			errs = append(errs, errors.WithStack(err))
		}
	} else {
		if ns.Status.Phase == corev1.NamespaceTerminating {
			fmt.Printf("Velero installation namespace %q is terminating.\n", namespace)
		} else {
			err = client.CoreV1().Namespaces().Delete(ctx, ns.Name, metav1.DeleteOptions{})
			if err != nil {
				errs = append(errs, errors.WithStack(err))
			}
		}
	}

	// rolebinding
	crb := install.ClusterRoleBinding(namespace)
	if err := client.RbacV1().ClusterRoleBindings().Delete(ctx, crb.Name, metav1.DeleteOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			fmt.Printf("Velero installation rolebinding %q does not exist, skipping.\n", crb.Name)
		} else {
			errs = append(errs, errors.WithStack(err))
		}
	}

	// CRDs
	veleroLabels := labels.FormatLabels(install.Labels())
	crds, err := extensionsClient.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{
		LabelSelector: veleroLabels,
	})
	if err != nil {
		errs = append(errs, errors.WithStack(err))
	}
	if len(crds.Items) == 0 {
		fmt.Print("Velero CRDs do not exist, skipping.\n")
	} else {
		for _, removeCRD := range crds.Items {
			if err = extensionsClient.ApiextensionsV1().CustomResourceDefinitions().Delete(ctx, removeCRD.ObjectMeta.Name, metav1.DeleteOptions{}); err != nil {
				errs = append(errs, errors.WithStack(err))
			}
		}
	}

	if waitToTerminate && len(ns.Name) != 0 {
		fmt.Println("Waiting for Velero uninstall to complete. You may safely press ctrl-c to stop waiting - uninstall will continue in the background.")

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		checkFunc := func() {
			nsUpdated, _ := client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
			if err != nil {
				errs = append(errs, errors.WithStack(err))
			}

			if nsUpdated.Status.Phase == corev1.NamespaceTerminating {
				fmt.Print(".")
			}

			if nsUpdated.Status.Phase != corev1.NamespaceTerminating {
				fmt.Print("\n")
				cancel()
			}
		}

		wait.Until(checkFunc, 5*time.Millisecond, ctx.Done())

	}

	if kubeerrs.NewAggregate(errs) != nil {
		fmt.Printf("Errors while attempting to uninstall Velero: %q", kubeerrs.NewAggregate(errs))
		return kubeerrs.NewAggregate(errs)
	}

	fmt.Println("Velero uninstalled ⛵")
	return nil
}
