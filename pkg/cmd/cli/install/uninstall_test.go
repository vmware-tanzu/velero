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

package install

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/spf13/cobra"
	corev1api "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	kcapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/client"
	velerov1clientset "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/typed/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/install"
)

func TestInstallUninstallOptions_Run(t *testing.T) {
	t.Parallel()
	type fields struct {
		Uninstall                  bool
		PreserveUninstallNamespace bool
		ExpectedResourceCount      int
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "Installed velero removed cleanly with uninstall flag",
			fields: fields{
				Uninstall:                  true,
				PreserveUninstallNamespace: false,
				ExpectedResourceCount:      17,
			},
			wantErr: false,
		},
		{
			name: "Installed velero removed but namespace is kept with uninstall preserveNamespace flag",
			fields: fields{
				Uninstall:                  true,
				PreserveUninstallNamespace: true,
				ExpectedResourceCount:      17,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := NewInstallOptions()
			env := envtest.Environment{}
			cfg, err := env.Start()
			if err != nil {
				t.Errorf("Failed to start test environment: %v", err)
			}
			defer func() {
				if err := env.Stop(); err != nil {
					t.Errorf("Failed to stop test environment: %v", err)
				}
			}()
			kubeconfigBytes, err := KubeConfigFromREST(cfg)
			if err != nil {
				t.Errorf("Failed to generate kubeconfig: %v", err)
			}
			if os.Getenv("KUBECONFIG") != "" {
				panic(fmt.Sprintf("Unexpected KUBECONFIG is already set to %s", os.Getenv("KUBECONFIG")))
			}
			envKubeConfigFileName := kubeConfigBytesToTmpFile(t, kubeconfigBytes)
			defer func() {
				if err := os.Remove(envKubeConfigFileName); err != nil {
					t.Errorf("Failed to remove temp file: %v", err)
				}
			}()
			fmt.Printf("envKubeConfigFileName: %s\n", envKubeConfigFileName)
			os.Setenv("KUBECONFIG", envKubeConfigFileName)
			defer func() {
				if err := os.Unsetenv("KUBECONFIG"); err != nil {
					t.Errorf("Failed to unset KUBECONFIG: %v", err)
				}
			}()

			veleroConfig, err := client.LoadConfig()
			if err != nil {
				t.Errorf("Failed to load velero config: %v", err)
			}
			f := client.NewFactory("test", veleroConfig)
			if err := o.Run(&cobra.Command{}, f); (err != nil) != tt.wantErr {
				t.Errorf("InstallOptions.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			clientset, err := f.KubeClient()
			if err != nil {
				t.Errorf("Failed to get kube client: %v", err)
			}
			success, count := countInstalledResources(clientset, cfg, t)
			if !success {
				t.Error("Error counting installed resources", count, tt.fields.ExpectedResourceCount)
			}
			if count != tt.fields.ExpectedResourceCount {
				t.Errorf("Install did not create the expected number of resources. Expected %d, got %d", tt.fields.ExpectedResourceCount, count)
			}
			if tt.fields.Uninstall {
				// now we run install with the uninstall flag
				o.Uninstall = true
				o.PreserveUninstallNamespace = tt.fields.PreserveUninstallNamespace
				if err := o.Run(&cobra.Command{}, f); (err != nil) != tt.wantErr {
					t.Errorf("InstallOptions.Run() error = %v, wantErr %v", err, tt.wantErr)
				}
				time.Sleep(2 * time.Second)
				success, count := countInstalledResources(clientset, cfg, t)
				if !success {
					t.Error("Error counting installed resources")
				}
				if count != 0 && !tt.fields.PreserveUninstallNamespace {
					t.Errorf("Uninstall did not remove all resources. Expected 0, got %d", count)
				}
				if tt.fields.PreserveUninstallNamespace {
					if count != 1 {
						t.Errorf("Uninstall did not preserve namespace. Expected 1, got %d", count)
					}
					// put dummy secret in namespace to test that it is preserved
					secret := &corev1api.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dummy-secret",
							Namespace: velerov1.DefaultNamespace,
						},
						Data: map[string][]byte{
							"dummy-key": []byte("dummy-val"),
						},
					}
					_, err := clientset.CoreV1().Secrets(velerov1.DefaultNamespace).Create(context.Background(), secret, metav1.CreateOptions{})
					if err != nil {
						t.Errorf("Failed to create dummy secret in namespace that supposedly still exists: %v", err)
					}
				}
			}
		})
	}
}

func countInstalledResources(clientset kubernetes.Interface, cfg *rest.Config, t *testing.T) (bool, int) {
	count := 0
	apiextensionsv1client := apiextensionsv1.NewForConfigOrDie(cfg)
	veleroV1Client := velerov1clientset.NewForConfigOrDie(cfg)
	crds, err := apiextensionsv1client.CustomResourceDefinitions().List(context.Background(), metav1.ListOptions{
		LabelSelector: labels.FormatLabels(install.Labels()),
	})
	if err != nil {
		t.Errorf("Failed to list CRDs: %v", err)
	}
	count += len(crds.Items)
	for _, crd := range crds.Items {
		fmt.Printf("count CRD: %s\n", crd.Name)
	}
	ns, err := clientset.CoreV1().Namespaces().Get(context.Background(), velerov1.DefaultNamespace, metav1.GetOptions{})
	if err == nil {
		if ns.Status.Phase == corev1api.NamespaceActive {
			// Quirks of envtest, the namespace is never removed, but it is marked as Terminating
			// https://github.com/kubernetes-sigs/controller-runtime/issues/880
			count++
		}
	} else if err != nil && !k8serrors.IsNotFound(err) {
		t.Errorf("Failed to get namespace: %v", err)
	}
	crbs, err := clientset.RbacV1().ClusterRoleBindings().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Errorf("Failed to list cluster role bindings: %v", err)
	}
	for _, crb := range crbs.Items {
		if len(crb.Subjects) > 0 && crb.Subjects[0].Name == "velero" {
			if crb.Subjects[0].Namespace == velerov1.DefaultNamespace {
				count++
				fmt.Printf("count CRB: %s\n", crb.Name)
				break
			}
		}
	}
	_, err = clientset.CoreV1().ServiceAccounts(velerov1.DefaultNamespace).Get(context.Background(), "velero", metav1.GetOptions{})
	if err == nil {
		count++
	} else if err != nil && !k8serrors.IsNotFound(err) {
		t.Errorf("Failed to get service account: %v", err)
	}
	_, err = veleroV1Client.BackupStorageLocations(velerov1.DefaultNamespace).Get(context.Background(), "default", metav1.GetOptions{})
	if err == nil {
		count++
	} else if err != nil && !k8serrors.IsNotFound(err) {
		t.Errorf("Failed to get backup storage location: %v", err)
	}
	_, err = veleroV1Client.VolumeSnapshotLocations(velerov1.DefaultNamespace).Get(context.Background(), "default", metav1.GetOptions{})
	if err == nil {
		count++
	} else if err != nil && !k8serrors.IsNotFound(err) {
		t.Errorf("Failed to get volume snapshot location: %v", err)
	}
	deployments, err := clientset.AppsV1().Deployments(velerov1.DefaultNamespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Errorf("Failed to list deployments: %v", err)
	}
	count += len(deployments.Items)
	for _, deployment := range deployments.Items {
		fmt.Printf("count deployment: %s\n", deployment.Name)
	}
	daemonsets, err := clientset.AppsV1().DaemonSets(velerov1.DefaultNamespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Errorf("Failed to list daemonsets: %v", err)
	}
	count += len(daemonsets.Items)
	for _, daemonset := range daemonsets.Items {
		fmt.Printf("count daemonset: %s\n", daemonset.Name)
	}
	// count the number of resources created
	return true, count
}

func kubeConfigBytesToTmpFile(t *testing.T, kubeconfigBytes []byte) string {
	tmpFile, err := os.CreateTemp("", "kubeconfig")
	if err != nil {
		t.Errorf("Failed to create temp file: %v", err)
	}
	defer tmpFile.Close()
	if _, err := tmpFile.Write(kubeconfigBytes); err != nil {
		t.Errorf("Failed to write kubeconfig to temp file: %v", err)
	}
	return tmpFile.Name()
}

/**
 * Snippets from controller-runtime@v0.12.2/pkg/internal/testing/controlplane/kubectl.go
 */
const (
	envtestName = "envtest"
)

// KubeConfigFromREST reverse-engineers a kubeconfig file from a rest.Config.
// The options are tailored towards the rest.Configs we generate, so they're
// not broadly applicable.
//
// This is not intended to be exposed beyond internal for the above reasons.
func KubeConfigFromREST(cfg *rest.Config) ([]byte, error) {
	kubeConfig := kcapi.NewConfig()
	protocol := "https"
	if !rest.IsConfigTransportTLS(*cfg) {
		protocol = "http"
	}

	// cfg.Host is a URL, so we need to parse it so we can properly append the API path
	baseURL, err := url.Parse(cfg.Host)
	if err != nil {
		return nil, fmt.Errorf("unable to interpret config's host value as a URL: %w", err)
	}

	kubeConfig.Clusters[envtestName] = &kcapi.Cluster{
		// TODO(directxman12): if client-go ever decides to expose defaultServerUrlFor(config),
		// we can just use that.  Note that this is not the same as the public DefaultServerURL,
		// which requires us to pass a bunch of stuff in manually.
		Server:                   (&url.URL{Scheme: protocol, Host: baseURL.Host, Path: cfg.APIPath}).String(),
		CertificateAuthorityData: cfg.CAData,
	}
	kubeConfig.AuthInfos[envtestName] = &kcapi.AuthInfo{
		// try to cover all auth strategies that aren't plugins
		ClientCertificateData: cfg.CertData,
		ClientKeyData:         cfg.KeyData,
		Token:                 cfg.BearerToken,
		Username:              cfg.Username,
		Password:              cfg.Password,
	}
	kcCtx := kcapi.NewContext()
	kcCtx.Cluster = envtestName
	kcCtx.AuthInfo = envtestName
	kubeConfig.Contexts[envtestName] = kcCtx
	kubeConfig.CurrentContext = envtestName

	contents, err := clientcmd.Write(*kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to serialize kubeconfig file: %w", err)
	}
	return contents, nil
}
