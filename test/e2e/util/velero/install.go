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

package velero

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/vmware-tanzu/velero/pkg/cmd/cli/install"
	velerexec "github.com/vmware-tanzu/velero/pkg/util/exec"
	. "github.com/vmware-tanzu/velero/test/e2e"
	. "github.com/vmware-tanzu/velero/test/e2e/util/k8s"
)

// we provide more install options other than the standard install.InstallOptions in E2E test
type installOptions struct {
	*install.InstallOptions
	RegistryCredentialFile string
	ResticHelperImage      string
}

func VeleroInstall(ctx context.Context, veleroCfg *VerleroConfig, useVolumeSnapshots bool) error {
	if veleroCfg.CloudProvider != "kind" {
		if veleroCfg.ObjectStoreProvider != "" {
			return errors.New("For cloud platforms, object store plugin cannot be overridden") // Can't set an object store provider that is different than your cloud
		}
		veleroCfg.ObjectStoreProvider = veleroCfg.CloudProvider
	} else {
		if veleroCfg.ObjectStoreProvider == "" {
			return errors.New("No object store provider specified - must be specified when using kind as the cloud provider") // Gotta have an object store provider
		}
	}

	providerPluginsTmp, err := getProviderPlugins(ctx, veleroCfg.VeleroCLI, veleroCfg.ObjectStoreProvider, veleroCfg.Plugins, veleroCfg.Features)
	if err != nil {
		return errors.WithMessage(err, "Failed to get provider plugins")
	}
	err = EnsureClusterExists(ctx)
	if err != nil {
		return errors.WithMessage(err, "Failed to ensure Kubernetes cluster exists")
	}

	// TODO - handle this better
	if veleroCfg.CloudProvider == "vsphere" {
		// We overrider the ObjectStoreProvider here for vSphere because we want to use the aws plugin for the
		// backup, but needed to pick up the provider plugins earlier.  vSphere plugin no longer needs a Volume
		// Snapshot location specified
		veleroCfg.ObjectStoreProvider = "aws"
		if err := configvSpherePlugin(*veleroCfg.ClientToInstallVelero); err != nil {
			return errors.WithMessagef(err, "Failed to config vsphere plugin")
		}
	}

	veleroInstallOptions, err := getProviderVeleroInstallOptions(veleroCfg.ObjectStoreProvider, veleroCfg.CloudCredentialsFile, veleroCfg.BSLBucket,
		veleroCfg.BSLPrefix, veleroCfg.BSLConfig, veleroCfg.VSLConfig, providerPluginsTmp, veleroCfg.Features)
	if err != nil {
		return errors.WithMessagef(err, "Failed to get Velero InstallOptions for plugin provider %s", veleroCfg.ObjectStoreProvider)
	}
	veleroInstallOptions.UseVolumeSnapshots = useVolumeSnapshots
	veleroInstallOptions.UseNodeAgent = !useVolumeSnapshots
	veleroInstallOptions.Image = veleroCfg.VeleroImage
	veleroInstallOptions.Namespace = veleroCfg.VeleroNamespace
	GCFrequency, _ := time.ParseDuration(veleroCfg.GCFrequency)
	veleroInstallOptions.GarbageCollectionFrequency = GCFrequency

	err = installVeleroServer(ctx, veleroCfg.VeleroCLI, &installOptions{
		InstallOptions:         veleroInstallOptions,
		RegistryCredentialFile: veleroCfg.RegistryCredentialFile,
		ResticHelperImage:      veleroCfg.ResticHelperImage,
	})
	if err != nil {
		return errors.WithMessagef(err, "Failed to install Velero in the cluster")
	}

	return nil
}

//configvSpherePlugin refers to https://github.com/vmware-tanzu/velero-plugin-for-vsphere/blob/v1.3.0/docs/vanilla.md
func configvSpherePlugin(cli TestClient) error {
	var err error
	vsphereSecret := "velero-vsphere-config-secret"
	configmaptName := "velero-vsphere-plugin-config"
	if err := clearupvSpherePluginConfig(cli.ClientGo, VeleroCfg.VeleroNamespace, vsphereSecret, configmaptName); err != nil {
		return errors.WithMessagef(err, "Failed to clear up vsphere plugin config %s namespace", VeleroCfg.VeleroNamespace)
	}
	if err := CreateNamespace(context.Background(), cli, VeleroCfg.VeleroNamespace); err != nil {
		return errors.WithMessagef(err, "Failed to create Velero %s namespace", VeleroCfg.VeleroNamespace)
	}
	if err := CreateVCCredentialSecret(cli.ClientGo, VeleroCfg.VeleroNamespace); err != nil {
		return errors.WithMessagef(err, "Failed to create virtual center credential secret in %s namespace", VeleroCfg.VeleroNamespace)
	}
	if err := WaitForSecretsComplete(cli.ClientGo, VeleroCfg.VeleroNamespace, vsphereSecret); err != nil {
		return errors.Wrap(err, "Failed to ensure velero-vsphere-config-secret secret completion in namespace kube-system")
	}
	_, err = CreateConfigMap(cli.ClientGo, VeleroCfg.VeleroNamespace, configmaptName, map[string]string{
		"cluster_flavor":           "VANILLA",
		"vsphere_secret_name":      vsphereSecret,
		"vsphere_secret_namespace": VeleroCfg.VeleroNamespace,
	})
	if err != nil {
		return errors.WithMessagef(err, "Failed to create velero-vsphere-plugin-config configmap in %s namespace", VeleroCfg.VeleroNamespace)
	}
	fmt.Println("configvSpherePlugin: WaitForConfigMapComplete")
	err = WaitForConfigMapComplete(cli.ClientGo, VeleroCfg.VeleroNamespace, configmaptName)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to ensure configmap %s completion in namespace: %s", configmaptName, VeleroCfg.VeleroNamespace))
	}
	return nil
}

func clearupvSpherePluginConfig(c clientset.Interface, ns, secretName, configMapName string) error {
	//clear secret
	_, err := GetSecret(c, ns, secretName)
	if err == nil { //exist
		if err := WaitForSecretDelete(c, ns, secretName); err != nil {
			return errors.WithMessagef(err, "Failed to clear up vsphere plugin secret in %s namespace", ns)
		}
	} else if !apierrors.IsNotFound(err) {
		return errors.WithMessagef(err, "Failed to retrieve vsphere plugin secret in %s namespace", ns)
	}

	//clear configmap
	_, err = GetConfigmap(c, ns, configMapName)
	if err == nil {
		if err := WaitForConfigmapDelete(c, ns, configMapName); err != nil {
			return errors.WithMessagef(err, "Failed to clear up vsphere plugin configmap in %s namespace", ns)
		}
	} else if !apierrors.IsNotFound(err) {
		return errors.WithMessagef(err, "Failed to retrieve vsphere plugin configmap in %s namespace", ns)
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
	if len(options.Image) > 0 {
		args = append(args, "--image", options.Image)
	}
	if options.UseNodeAgent {
		args = append(args, "--use-node-agent")
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
		if strings.EqualFold(options.Features, "EnableCSI") && options.UseVolumeSnapshots {
			if strings.EqualFold(options.ProviderName, "Azure") {
				if err := KubectlApplyByFile(ctx, "util/csi/AzureVolumeSnapshotClass.yaml"); err != nil {
					return err
				}
			}

		}
	}
	if options.GarbageCollectionFrequency > 0 {
		args = append(args, fmt.Sprintf("--garbage-collection-frequency=%v", options.GarbageCollectionFrequency))
	}

	if err := createVelereResources(ctx, cli, namespace, args, options.RegistryCredentialFile, options.ResticHelperImage); err != nil {
		return err
	}

	return waitVeleroReady(ctx, namespace, options.UseNodeAgent)
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

	if err = patchResources(ctx, resources, namespace, registryCredentialFile, VeleroCfg.ResticHelperImage); err != nil {
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
	if len(VeleroCfg.ResticHelperImage) > 0 {
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
				"image": VeleroCfg.ResticHelperImage,
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

func waitVeleroReady(ctx context.Context, namespace string, useNodeAgent bool) error {
	fmt.Println("Waiting for Velero deployment to be ready.")
	// when doing upgrade by the "kubectl apply" the command "kubectl wait --for=condition=available deployment/velero -n velero --timeout=600s" returns directly
	// use "rollout status" instead to avoid this. For more detail information, refer to https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#complete-deployment
	stdout, stderr, err := velerexec.RunCommand(exec.CommandContext(ctx, "kubectl", "rollout", "status",
		"deployment/velero", "-n", namespace))
	if err != nil {
		return errors.Wrapf(err, "fail to wait for the velero deployment ready, stdout=%s, stderr=%s", stdout, stderr)
	}

	if useNodeAgent {
		fmt.Println("Waiting for node-agent daemonset to be ready.")
		err := wait.PollImmediate(5*time.Second, 1*time.Minute, func() (bool, error) {
			stdout, stderr, err := velerexec.RunCommand(exec.CommandContext(ctx, "kubectl", "get", "daemonset/node-agent",
				"-o", "json", "-n", namespace))
			if err != nil {
				return false, errors.Wrapf(err, "failed to get the node-agent daemonset, stdout=%s, stderr=%s", stdout, stderr)
			}
			daemonset := &apps.DaemonSet{}
			if err = json.Unmarshal([]byte(stdout), daemonset); err != nil {
				return false, errors.Wrapf(err, "failed to unmarshal the node-agent daemonset")
			}
			if daemonset.Status.DesiredNumberScheduled == daemonset.Status.NumberAvailable {
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			return errors.Wrap(err, "fail to wait for the node-agent ready")
		}
	}

	fmt.Printf("Velero is installed and ready to be tested in the %s namespace! ⛵ \n", namespace)
	return nil
}

func VeleroUninstall(ctx context.Context, cli, namespace string) error {
	stdout, stderr, err := velerexec.RunCommand(exec.CommandContext(ctx, cli, "uninstall", "--force", "-n", namespace))
	if err != nil {
		return errors.Wrapf(err, "failed to uninstall velero, stdout=%s, stderr=%s", stdout, stderr)
	}
	fmt.Println("Velero uninstalled ⛵")
	return nil
}
