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

	appsv1api "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	kubeerrs "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/cli"
	"github.com/vmware-tanzu/velero/pkg/controller"
	"github.com/vmware-tanzu/velero/pkg/install"
	kubeutil "github.com/vmware-tanzu/velero/pkg/util/kube"
)

var gracefulDeletionMaximumDuration = 1 * time.Minute

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
	// Deal with resources with attached finalizers to ensure proper handling of those finalizers.
	if err := deleteResourcesWithFinalizer(ctx, kbClient, namespace); err != nil {
		return errors.Wrap(err, "Fail to remove finalizer from restores")
	}

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
	fmt.Println()
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

// A few things needed to be noticed here:
// 1. When we delete resources with attached finalizers, the corresponding controller will deal with the finalizer then resources can be deleted successfully.
// So it is important to delete these resources before deleting the pod that runs that controller.
// 2. The controller may encounter errors while handling the finalizer, in such case, the controller will keep trying until it succeeds.
// So it is important to set a timeout, once the process exceed the timeout, we will forcedly delete the resources by removing the finalizer from them,
// otherwise the deletion process may get stuck indefinitely.
// 3. There is only restore finalizer supported as of v1.12. If any new finalizers are added in the future, the corresponding deletion logic can be
// incorporated into this function.
func deleteResourcesWithFinalizer(ctx context.Context, kbClient kbclient.Client, namespace string) error {
	fmt.Println("Waiting for resource with attached finalizer to be deleted")
	return deleteRestore(ctx, kbClient, namespace)
}

func deleteRestore(ctx context.Context, kbClient kbclient.Client, namespace string) error {
	// Check if restore crd exists, if it does not exist, return immediately.
	var err error
	v1crd := &apiextv1.CustomResourceDefinition{}
	key := kbclient.ObjectKey{Name: "restores.velero.io"}
	if err = kbClient.Get(ctx, key, v1crd); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		} else {
			return errors.Wrap(err, "Error getting restore crd")
		}
	}

	// First attempt to gracefully delete all the restore within a specified time frame, If the process exceeds the timeout limit,
	// it is likely that there may be errors during the finalization of restores. In such cases, we should proceed with forcefully deleting the restores.
	err = gracefullyDeleteRestore(ctx, kbClient, namespace)
	if err != nil && err != wait.ErrWaitTimeout {
		return errors.Wrap(err, "Error deleting restores")
	}
	if err == wait.ErrWaitTimeout {
		err = forcedlyDeleteRestore(ctx, kbClient, namespace)
		if err != nil {
			return errors.Wrap(err, "Error deleting restores")
		}
	}

	return nil
}

func gracefullyDeleteRestore(ctx context.Context, kbClient kbclient.Client, namespace string) error {
	var err error
	restoreList := &velerov1api.RestoreList{}
	if err = kbClient.List(ctx, restoreList, &kbclient.ListOptions{Namespace: namespace}); err != nil {
		return errors.Wrap(err, "Error getting restores during graceful deletion")
	}

	for i := range restoreList.Items {
		if err = kbClient.Delete(ctx, &restoreList.Items[i]); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return errors.Wrap(err, "Error deleting restores during graceful deletion")
		}
	}

	// Wait for the deletion of all the restores within a specified time frame
	err = wait.PollImmediate(time.Second, gracefulDeletionMaximumDuration, func() (bool, error) {
		restoreList := &velerov1api.RestoreList{}
		if errList := kbClient.List(ctx, restoreList, &kbclient.ListOptions{Namespace: namespace}); errList != nil {
			return false, errList
		}

		if len(restoreList.Items) > 0 {
			fmt.Print(".")
			return false, nil
		} else {
			return true, nil
		}
	})

	return err
}

func forcedlyDeleteRestore(ctx context.Context, kbClient kbclient.Client, namespace string) error {
	// Delete velero deployment first in case:
	// 1. finalizers will be added back by restore controller after they are removed at next step;
	// 2. new restores attached with finalizer will be created by restore controller after we remove all the restores' finalizer at next step;
	deploy := &appsv1api.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "velero",
			Name:      namespace,
		},
	}

	err := kbClient.Delete(ctx, deploy)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrap(err, "Error deleting velero deployment during force deletion")
	}

	ctxc, cancel := context.WithCancel(ctx)
	defer cancel()

	checkFunc := func() {
		deploy := &appsv1api.Deployment{}
		key := kbclient.ObjectKey{Namespace: namespace, Name: "velero"}

		if err = kbClient.Get(ctxc, key, deploy); err != nil {
			if apierrors.IsNotFound(err) {
				err = nil
			}
			cancel()
			return
		}
	}

	// Wait until velero deployment are deleted.
	wait.Until(checkFunc, 100*time.Millisecond, ctxc.Done())
	if err != nil {
		return errors.Wrap(err, "Error deleting velero deployment during force deletion")
	}

	// Remove all the restores' finalizer so they can be deleted during the deletion of velero namespace.
	restoreList := &velerov1api.RestoreList{}
	if err := kbClient.List(ctx, restoreList, &kbclient.ListOptions{Namespace: namespace}); err != nil {
		return errors.Wrap(err, "Error getting restores during force deletion")
	}

	for i := range restoreList.Items {
		if controllerutil.ContainsFinalizer(&restoreList.Items[i], controller.ExternalResourcesFinalizer) {
			update := &restoreList.Items[i]
			original := update.DeepCopy()
			controllerutil.RemoveFinalizer(update, controller.ExternalResourcesFinalizer)
			if err := kubeutil.PatchResource(original, update, kbClient); err != nil {
				return errors.Wrap(err, "Error removing restore finalizer during force deletion")
			}
		}
	}

	return nil
}
