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

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/vmware-tanzu/velero/pkg/cmd/cli/install"
	velerexec "github.com/vmware-tanzu/velero/pkg/util/exec"
)

// we provide more install options other than the standard install.InstallOptions in E2E test
type installOptions struct {
	*install.InstallOptions
	RegistryCredentialFile string
	ResticHelperImage      string
}

// TODO too many parameters for this function, better to make it a structure, we can introduces a structure `config` for the E2E to hold all configuration items
func veleroInstall(ctx context.Context, cli, veleroImage string, resticHelperImage string, veleroNamespace string, cloudProvider string, objectStoreProvider string, useVolumeSnapshots bool,
	cloudCredentialsFile string, bslBucket string, bslPrefix string, bslConfig string, vslConfig string,
	crdsVersion string, features string, registryCredentialFile string) error {

	if cloudProvider != "kind" {
		if objectStoreProvider != "" {
			return errors.New("For cloud platforms, object store plugin cannot be overridden") // Can't set an object store provider that is different than your cloud
		}
		objectStoreProvider = cloudProvider
	} else {
		if objectStoreProvider == "" {
			return errors.New("No object store provider specified - must be specified when using kind as the cloud provider") // Gotta have an object store provider
		}
	}

	// Fetch the plugins for the provider before checking for the object store provider below.
	providerPlugins := getProviderPlugins(objectStoreProvider)

	// TODO - handle this better
	if cloudProvider == "vsphere" {
		// We overrider the objectStoreProvider here for vSphere because we want to use the aws plugin for the
		// backup, but needed to pick up the provider plugins earlier.  vSphere plugin no longer needs a Volume
		// Snapshot location specified
		objectStoreProvider = "aws"
	}
	err := ensureClusterExists(ctx)
	if err != nil {
		return errors.WithMessage(err, "Failed to ensure Kubernetes cluster exists")
	}

	veleroInstallOptions, err := getProviderVeleroInstallOptions(objectStoreProvider, cloudCredentialsFile, bslBucket,
		bslPrefix, bslConfig, vslConfig, providerPlugins, features)
	if err != nil {
		return errors.WithMessagef(err, "Failed to get Velero InstallOptions for plugin provider %s", objectStoreProvider)
	}
	veleroInstallOptions.UseVolumeSnapshots = useVolumeSnapshots
	veleroInstallOptions.UseRestic = !useVolumeSnapshots
	veleroInstallOptions.Image = veleroImage
	veleroInstallOptions.CRDsVersion = crdsVersion
	veleroInstallOptions.Namespace = veleroNamespace

	err = installVeleroServer(ctx, cli, &installOptions{
		InstallOptions:         veleroInstallOptions,
		RegistryCredentialFile: registryCredentialFile,
		ResticHelperImage:      resticHelperImage,
	})
	if err != nil {
		return errors.WithMessagef(err, "Failed to install Velero in the cluster")
	}

	return nil
}

func installVeleroServer(ctx context.Context, cli string, options *installOptions) error {
	args := []string{"install"}
	namespace := "velero"
	if len(options.Namespace) > 0 {
		args = append(args, "--namespace", options.Namespace)
		namespace = options.Namespace
	}
	if len(options.CRDsVersion) > 0 {
		args = append(args, "--crds-version", options.CRDsVersion)
	}
	if len(options.Image) > 0 {
		args = append(args, "--image", options.Image)
	}
	if options.UseRestic {
		args = append(args, "--use-restic")
	}
	if options.UseVolumeSnapshots {
		args = append(args, "--use-volume-snapshots")
	}
	if len(options.ProviderName) > 0 {
		args = append(args, "--provider", options.ProviderName)
	}
	if len(options.BackupStorageConfig.Data()) > 0 {
		args = append(args, "--backup-location-config", options.BackupStorageConfig.String())
	}
	if len(options.BucketName) > 0 {
		args = append(args, "--bucket", options.BucketName)
	}
	if len(options.Prefix) > 0 {
		args = append(args, "--prefix", options.Prefix)
	}
	if len(options.SecretFile) > 0 {
		args = append(args, "--secret-file", options.SecretFile)
	}
	if len(options.VolumeSnapshotConfig.Data()) > 0 {
		args = append(args, "--snapshot-location-config", options.VolumeSnapshotConfig.String())
	}
	if len(options.Plugins) > 0 {
		args = append(args, "--plugins", options.Plugins.String())
	}
	if len(options.Features) > 0 {
		args = append(args, "--features", options.Features)
	}

	if err := createVelereResources(ctx, cli, namespace, args, options.RegistryCredentialFile, options.ResticHelperImage); err != nil {
		return err
	}

	return waitVeleroReady(ctx, namespace, options.UseRestic)
}

func createVelereResources(ctx context.Context, cli, namespace string, args []string, registryCredentialFile, resticHelperImage string) error {
	args = append(args, "--dry-run", "--output", "json", "--crds-only")

	// get the CRD definitions
	cmd := exec.CommandContext(ctx, cli, args...)
	fmt.Printf("Running cmd %q \n", cmd.String())
	stdout, stderr, err := velerexec.RunCommand(cmd)
	if err != nil {
		return errors.Wrapf(err, "failed to run velero install dry run command with \"--crds-only\" option, stdout=%s, stderr=%s", stdout, stderr)
	}

	// apply the CRDs first
	cmd = exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	cmd.Stdin = bytes.NewBufferString(stdout)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Printf("Applying velero CRDs...\n")
	if err = cmd.Run(); err != nil {
		return errors.Wrapf(err, "failed to apply the CRDs")
	}

	// wait until the CRDs are ready
	cmd = exec.CommandContext(ctx, "kubectl", "wait", "--for", "condition=established", "-f", "-")
	cmd.Stdin = bytes.NewBufferString(stdout)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Printf("Waiting velero CRDs ready...\n")
	if err = cmd.Run(); err != nil {
		return errors.Wrapf(err, "failed to wait the CRDs be ready")
	}

	// remove the "--crds-only" option from the args
	args = args[:len(args)-1]
	cmd = exec.CommandContext(ctx, cli, args...)
	fmt.Printf("Running cmd %q \n", cmd.String())
	stdout, stderr, err = velerexec.RunCommand(cmd)
	if err != nil {
		return errors.Wrapf(err, "failed to run velero install dry run command, stdout=%s, stderr=%s", stdout, stderr)
	}

	resources := &unstructured.UnstructuredList{}
	if err := json.Unmarshal([]byte(stdout), resources); err != nil {
		return errors.Wrapf(err, "failed to unmarshal the resources: %s", string(stdout))
	}

	if err = patchResources(ctx, resources, namespace, registryCredentialFile, resticHelperImage); err != nil {
		return errors.Wrapf(err, "failed to patch resources")
	}

	data, err := json.Marshal(resources)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal resources")
	}

	cmd = exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	cmd.Stdin = bytes.NewReader(data)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Printf("Running cmd %q \n", cmd.String())
	if err = cmd.Run(); err != nil {
		return errors.Wrapf(err, "failed to apply velere resources")
	}

	return nil
}

// patch the velero resources
func patchResources(ctx context.Context, resources *unstructured.UnstructuredList, namespace, registryCredentialFile, resticHelperImage string) error {
	// apply the image pull secret to avoid the image pull limit of Docker Hub
	if len(registryCredentialFile) > 0 {
		credential, err := ioutil.ReadFile(registryCredentialFile)
		if err != nil {
			return errors.Wrapf(err, "failed to read the registry credential file %s", registryCredentialFile)
		}

		imagePullSecret := corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: corev1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "image-pull-secret",
				Namespace: namespace,
			},
			Type: corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{
				".dockerconfigjson": credential,
			},
		}

		for resourceIndex, resource := range resources.Items {
			if resource.GetKind() == "ServiceAccount" && resource.GetName() == "velero" {
				resource.Object["imagePullSecrets"] = []map[string]interface{}{
					{
						"name": "image-pull-secret",
					},
				}
				resources.Items[resourceIndex] = resource
				fmt.Printf("image pull secret %q set for velero serviceaccount \n", "image-pull-secret")
				break
			}
		}

		un, err := toUnstructured(imagePullSecret)
		if err != nil {
			return errors.Wrapf(err, "failed to convert pull secret to unstructure")
		}
		resources.Items = append(resources.Items, un)
	}

	// customize the restic restore helper image
	if len(resticHelperImage) > 0 {
		restoreActionConfig := corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: corev1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "restic-restore-action-config",
				Namespace: namespace,
				Labels: map[string]string{
					"velero.io/plugin-config": "",
					"velero.io/restic":        "RestoreItemAction",
				},
			},
			Data: map[string]string{
				"image": resticHelperImage,
			},
		}

		un, err := toUnstructured(restoreActionConfig)
		if err != nil {
			return errors.Wrapf(err, "failed to convert restore action config to unstructure")
		}
		resources.Items = append(resources.Items, un)
		fmt.Printf("the restic restore helper image is set by the configmap %q \n", "restic-restore-action-config")
	}

	return nil
}

func toUnstructured(res interface{}) (unstructured.Unstructured, error) {
	un := unstructured.Unstructured{}
	data, err := json.Marshal(res)
	if err != nil {
		return un, err
	}
	err = json.Unmarshal(data, &un)
	return un, err
}

func waitVeleroReady(ctx context.Context, namespace string, useRestic bool) error {
	fmt.Println("Waiting for Velero deployment to be ready.")
	stdout, stderr, err := velerexec.RunCommand(exec.CommandContext(ctx, "kubectl", "wait", "--for=condition=available",
		"deployment/velero", "-n", namespace, "--timeout=600s"))
	if err != nil {
		return errors.Wrapf(err, "fail to wait for the velero deployment ready, stdout=%s, stderr=%s", stdout, stderr)
	}

	if useRestic {
		fmt.Println("Waiting for Velero restic daemonset to be ready.")
		wait.PollImmediate(5*time.Second, 1*time.Minute, func() (bool, error) {
			stdout, stderr, err := velerexec.RunCommand(exec.CommandContext(ctx, "kubectl", "get", "daemonset/restic",
				"-o", "json", "-n", namespace))
			if err != nil {
				return false, errors.Wrapf(err, "failed to get the restic daemonset, stdout=%s, stderr=%s", stdout, stderr)
			}
			restic := &apps.DaemonSet{}
			if err = json.Unmarshal([]byte(stdout), restic); err != nil {
				return false, errors.Wrapf(err, "failed to unmarshal the restic daemonset")
			}
			if restic.Status.DesiredNumberScheduled == restic.Status.NumberAvailable {
				return true, nil
			}
			return false, nil
		})
	}

	fmt.Printf("Velero is installed and ready to be tested in the %s namespace! ⛵ \n", namespace)
	return nil
}

func veleroUninstall(ctx context.Context, cli, namespace string) error {
	stdout, stderr, err := velerexec.RunCommand(exec.CommandContext(ctx, cli, "uninstall", "--force", "-n", namespace))
	if err != nil {
		return errors.Wrapf(err, "failed to uninstall velero, stdout=%s, stderr=%s", stdout, stderr)
	}
	fmt.Println("Velero uninstalled ⛵")
	return nil
}
