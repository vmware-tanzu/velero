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
	"github.com/vmware-tanzu/velero/pkg/client"
	"k8s.io/client-go/kubernetes"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type testClient struct {
	Kubebuilder    kbclient.Client
	ClientGo       kubernetes.Interface
	DynamicFactory client.DynamicFactory
}

func newTestClient() (testClient, error) {
	config, err := client.LoadConfig()
	if err != nil {
		return testClient{}, err
	}

	f := client.NewFactory("e2e", config)

	clientGo, err := f.KubeClient()
	if err != nil {
		return testClient{}, err
	}

	kb, err := f.KubebuilderClient()
	if err != nil {
		return testClient{}, err
	}

	dynamicClient, err := f.DynamicClient()
	if err != nil {
		return testClient{}, err
	}

	factory := client.NewDynamicFactory(dynamicClient)

	return testClient{
		Kubebuilder:    kb,
		ClientGo:       clientGo,
		DynamicFactory: factory,
	}, nil
}
