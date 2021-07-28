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
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	kubeerrs "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/cli"
	"github.com/vmware-tanzu/velero/pkg/install"
)

// uninstallOptions collects all the options for uninstalling Velero from a Kubernetes cluster.
type uninstallOptions struct {
	wait  bool // deprecated
	force bool
}

// BindFlags adds command line values to the options struct.
func (o *uninstallOptions) BindFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&o.wait, "wait", o.wait, "Wait for Velero uninstall to be ready. Optional. Deprecated.")
	flags.BoolVar(&o.force, "force", o.force, "Forces the Velero uninstall. Optional.")
}

// NewCommand creates a cobra command.
func NewCommand(f client.Factory) *cobra.Command {
	o := &uninstallOptions{}

	c := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall Velero",
		Long: `Uninstall Velero along with the CRDs and clusterrolebinding.

The '--namespace' flag can be used to specify the namespace where velero is installed (default: velero).
Use '--force' to skip the prompt confirming if you want to uninstall Velero.
		`,
		Example: ` # velero uninstall --namespace staging`,
		Run: func(c *cobra.Command, args []string) {
			if o.wait {
				fmt.Println("Warning: the \"--wait\" option is deprecated and will be removed in a future release. The uninstall command always waits for the uninstall to complete.")
			}

			// Confirm if not asked to force-skip confirmation
			if !o.force {
				fmt.Println("You are about to uninstall Velero.")
				if !cli.GetConfirmation() {
					// Don't do anything unless we get confirmation
					return
				}
			}

			kbClient, err := f.KubebuilderClient()
			cmd.CheckError(err)
			cmd.CheckError(Run(context.Background(), kbClient, f.Namespace()))
		},
	}

	o.BindFlags(c.Flags())
	return c
}

// Run removes all components that were deployed using the Velero install command
func Run(ctx context.Context, kbClient kbclient.Client, namespace string) error {
	// The CRDs cannot be removed until the namespace is deleted to avoid the problem in issue #3974 so if the namespace deletion fails we error out here
	if err := deleteNamespace(ctx, kbClient, namespace); err != nil {
		fmt.Printf("Errors while attempting to uninstall Velero: %q \n", err)
		return err
	}

	var errs []error

	// ClusterRoleBinding
	crb := install.ClusterRoleBinding(namespace)
	key := kbclient.ObjectKey{Name: crb.Name}
	if err := kbClient.Get(ctx, key, crb); err != nil {
		if apierrors.IsNotFound(err) {
			fmt.Printf("Velero ClusterRoleBinding %q does not exist, skipping.\n", crb.Name)
		} else {
			errs = append(errs, errors.WithStack(err))
		}
	} else {
		if err := kbClient.Delete(ctx, crb); err != nil {
			errs = append(errs, errors.WithStack(err))
		}
	}

	// CRDs

	veleroLabelSelector := labels.SelectorFromSet(install.Labels())
	opts := []kbclient.DeleteAllOfOption{
		kbclient.InNamespace(namespace),
		kbclient.MatchingLabelsSelector{
			Selector: veleroLabelSelector,
		},
	}
	v1CRDsRemoved := false
	v1crd := &apiextv1.CustomResourceDefinition{}
	if err := kbClient.DeleteAllOf(ctx, v1crd, opts...); err != nil {
		if meta.IsNoMatchError(err) {
			fmt.Println("V1 Velero CRDs not found, skipping...")
		} else {
			errs = append(errs, errors.WithStack(err))
		}
	} else {
		v1CRDsRemoved = true
	}

	// Remove any old Velero v1beta1 CRDs hanging around.
	v1beta1crd := &apiextv1beta1.CustomResourceDefinition{}
	if err := kbClient.DeleteAllOf(ctx, v1beta1crd, opts...); err != nil {
		if meta.IsNoMatchError(err) {
			if !v1CRDsRemoved {
				// Only mention this if there were no V1 CRDs removed
				fmt.Println("V1Beta1 Velero CRDs not found, skipping...")
			}
		} else {
			errs = append(errs, errors.WithStack(err))
		}
	}

	if kubeerrs.NewAggregate(errs) != nil {
		fmt.Printf("Errors while attempting to uninstall Velero: %q \n", kubeerrs.NewAggregate(errs))
		return kubeerrs.NewAggregate(errs)
	}

	fmt.Println("Velero uninstalled ⛵")
	return nil
}

func deleteNamespace(ctx context.Context, kbClient kbclient.Client, namespace string) error {
	ns := &corev1.Namespace{}
	key := kbclient.ObjectKey{Name: namespace}
	if err := kbClient.Get(ctx, key, ns); err != nil {
		if apierrors.IsNotFound(err) {
			fmt.Printf("Velero namespace %q does not exist, skipping.\n", namespace)
			return nil
		}
		return err
	}

	if err := kbClient.Delete(ctx, ns); err != nil {
		if apierrors.IsNotFound(err) {
			fmt.Printf("Velero namespace %q does not exist, skipping.\n", namespace)
			return nil
		}
		return err
	}

	fmt.Printf("Waiting for velero namespace %q to be deleted\n", namespace)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var err error
	checkFunc := func() {
		if err = kbClient.Get(ctx, key, ns); err != nil {
			if apierrors.IsNotFound(err) {
				fmt.Print("\n")
				err = nil
			}
			cancel()
			return
		}
		fmt.Print(".")
	}

	// Must wait until the namespace is deleted to avoid the issue https://github.com/vmware-tanzu/velero/issues/3974
	wait.Until(checkFunc, 5*time.Millisecond, ctx.Done())
	if err != nil {
		return err
	}

	fmt.Printf("Velero namespace %q deleted\n", namespace)
	return nil
}
