/*
Copyright 2017 the Heptio Ark contributors.

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

package client

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"k8s.io/client-go/kubernetes"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	clientset "github.com/heptio/ark/pkg/generated/clientset/versioned"
)

// Factory knows how to create an ArkClient and Kubernetes client.
type Factory interface {
	// BindFlags binds common flags (--kubeconfig, --namespace) to the passed-in FlagSet.
	BindFlags(flags *pflag.FlagSet)
	// Client returns an ArkClient. It uses the following priority to specify the cluster
	// configuration: --kubeconfig flag, KUBECONFIG environment variable, in-cluster configuration.
	Client() (clientset.Interface, error)
	// KubeClient returns a Kubernetes client. It uses the following priority to specify the cluster
	// configuration: --kubeconfig flag, KUBECONFIG environment variable, in-cluster configuration.
	KubeClient() (kubernetes.Interface, error)
	Namespace() string
}

type factory struct {
	flags       *pflag.FlagSet
	kubeconfig  string
	kubecontext string
	baseName    string
	namespace   string
}

// NewFactory returns a Factory.
func NewFactory(baseName string) Factory {
	f := &factory{
		flags:    pflag.NewFlagSet("", pflag.ContinueOnError),
		baseName: baseName,
	}

	if config, err := LoadConfig(); err == nil {
		f.namespace = config[ConfigKeyNamespace]
	} else {
		fmt.Fprintf(os.Stderr, "WARNING: error retrieving namespace from config file: %v\n", err)
	}

	if f.namespace == "" {
		f.namespace = v1.DefaultNamespace
	}

	f.flags.StringVar(&f.kubeconfig, "kubeconfig", "", "Path to the kubeconfig file to use to talk to the Kubernetes apiserver. If unset, try the environment variable KUBECONFIG, as well as in-cluster configuration")
	f.flags.StringVarP(&f.namespace, "namespace", "n", f.namespace, "The namespace in which Ark should operate")
	f.flags.StringVar(&f.kubecontext, "kubecontext", "", "The context to use to talk to the Kubernetes apiserver. If unset defaults to whatever your current-context is (kubectl config current-context)")

	return f
}

func (f *factory) BindFlags(flags *pflag.FlagSet) {
	flags.AddFlagSet(f.flags)
}

func (f *factory) Client() (clientset.Interface, error) {
	clientConfig, err := Config(f.kubeconfig, f.kubecontext, f.baseName)
	if err != nil {
		return nil, err
	}

	arkClient, err := clientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return arkClient, nil
}

func (f *factory) KubeClient() (kubernetes.Interface, error) {
	clientConfig, err := Config(f.kubeconfig, f.kubecontext, f.baseName)
	if err != nil {
		return nil, err
	}

	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return kubeClient, nil
}

func (f *factory) Namespace() string {
	return f.namespace
}
