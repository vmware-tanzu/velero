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

	"github.com/spf13/cobra"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	kcapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd/util/flag"
	velerov1clientset "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/typed/velero/v1"
)

func TestInstallOptions_Run(t *testing.T) {
	type fields struct {
		InstallAnnotations flag.Map
		InstallLabels      flag.Map
	}
	fooFlag := func() flag.Map {
		fooMap := flag.NewMap().WithEntryDelimiter(',').WithKeyValueDelimiter('=')
		err := fooMap.Set("foo=bar")
		if err != nil {
			panic(err)
		}
		return fooMap
	}()
	tests := []struct {
		name            string
		fields          fields
		wantErr         bool
		wantAnnotations map[string]string
		wantLabels      map[string]string
	}{
		{
			name: "Install labels and annotations are set",
			fields: fields{
				InstallAnnotations: fooFlag,
				InstallLabels:      fooFlag,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := NewInstallOptions()
			o.InstallAnnotations = tt.fields.InstallAnnotations
			o.InstallLabels = tt.fields.InstallLabels
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
			apiextensionsv1client := apiextensionsv1.NewForConfigOrDie(cfg)
			veleroV1Client := velerov1clientset.NewForConfigOrDie(cfg)
			crds, err := apiextensionsv1client.CustomResourceDefinitions().List(context.Background(), metav1.ListOptions{})
			if err != nil {
				t.Errorf("Failed to list CRDs: %v", err)
			}
			ns, err := clientset.CoreV1().Namespaces().Get(context.Background(), velerov1.DefaultNamespace, metav1.GetOptions{})
			if err != nil {
				t.Errorf("Failed to get namespace: %v", err)
			}
			crbs, err := clientset.RbacV1().ClusterRoleBindings().List(context.Background(), metav1.ListOptions{})
			if err != nil {
				t.Errorf("Failed to list cluster role bindings: %v", err)
			}
			sa, err := clientset.CoreV1().ServiceAccounts(velerov1.DefaultNamespace).Get(context.Background(), "velero", metav1.GetOptions{})
			if err != nil {
				t.Errorf("Failed to get service account: %v", err)
			}
			bsl, err := veleroV1Client.BackupStorageLocations(velerov1.DefaultNamespace).Get(context.Background(), "default", metav1.GetOptions{})
			if err != nil {
				t.Errorf("Failed to get backup storage location: %v", err)
			}
			vsl, err := veleroV1Client.VolumeSnapshotLocations(velerov1.DefaultNamespace).Get(context.Background(), "default", metav1.GetOptions{})
			if err != nil {
				t.Errorf("Failed to get volume snapshot location: %v", err)
			}
			deployments, err := clientset.AppsV1().Deployments(velerov1.DefaultNamespace).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				t.Errorf("Failed to list deployments: %v", err)
			}
			daemonsets, err := clientset.AppsV1().DaemonSets(velerov1.DefaultNamespace).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				t.Errorf("Failed to list daemonsets: %v", err)
			}
			// Check annotations and labels
			for k, v := range tt.wantAnnotations {
				if crds != nil {
					for _, crd := range crds.Items {
						if crd.Annotations[k] != v {
							t.Errorf("CRD annotations[%s] = %s, want %s", k, crd.Annotations[k], v)
						}
					}
				}
				if ns.Annotations[k] != v {
					t.Errorf("Namespace annotations[%s] = %s, want %s", k, ns.Annotations[k], v)
				}
				if crbs != nil {
					for _, crb := range crbs.Items {
						if crb.Annotations[k] != v {
							t.Errorf("CRB annotations[%s] = %s, want %s", k, crb.Annotations[k], v)
						}
					}
				}
				if sa.Annotations[k] != v {
					t.Errorf("ServiceAccount annotations[%s] = %s, want %s", k, sa.Annotations[k], v)
				}
				if bsl.Annotations[k] != v {
					t.Errorf("BackupStorageLocation annotations[%s] = %s, want %s", k, bsl.Annotations[k], v)
				}
				if vsl.Annotations[k] != v {
					t.Errorf("VolumeSnapshotLocation annotations[%s] = %s, want %s", k, vsl.Annotations[k], v)
				}
				for _, deployment := range deployments.Items {
					if deployment.Annotations[k] != v {
						t.Errorf("Deployment annotations[%s] = %s, want %s", k, deployment.Annotations[k], v)
					}
				}
				for _, daemonset := range daemonsets.Items {
					if daemonset.Annotations[k] != v {
						t.Errorf("Daemonset annotations[%s] = %s, want %s", k, daemonset.Annotations[k], v)
					}
				}
			}
			for k, v := range tt.wantLabels {
				if crds != nil {
					for _, crd := range crds.Items {
						if crd.Labels[k] != v {
							t.Errorf("CRD labels[%s] = %s, want %s", k, crd.Labels[k], v)
						}
					}
				}
				if ns.Labels[k] != v {
					t.Errorf("Namespace labels[%s] = %s, want %s", k, ns.Labels[k], v)
				}
				if crbs != nil {
					for _, crb := range crbs.Items {
						if crb.Labels[k] != v {
							t.Errorf("CRB labels[%s] = %s, want %s", k, crb.Labels[k], v)
						}
					}
				}
				if sa.Labels[k] != v {
					t.Errorf("ServiceAccount labels[%s] = %s, want %s", k, sa.Labels[k], v)
				}
				if bsl.Labels[k] != v {
					t.Errorf("BackupStorageLocation labels[%s] = %s, want %s", k, bsl.Labels[k], v)
				}
				if vsl.Labels[k] != v {
					t.Errorf("VolumeSnapshotLocation labels[%s] = %s, want %s", k, vsl.Labels[k], v)
				}
				for _, deployment := range deployments.Items {
					if deployment.Labels[k] != v {
						t.Errorf("Deployment labels[%s] = %s, want %s", k, deployment.Labels[k], v)
					}
				}
				for _, daemonset := range daemonsets.Items {
					if daemonset.Labels[k] != v {
						t.Errorf("Daemonset labels[%s] = %s, want %s", k, daemonset.Labels[k], v)
					}
				}
			}
		})
	}
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
