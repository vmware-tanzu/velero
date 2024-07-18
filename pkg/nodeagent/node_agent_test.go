/*
Copyright The Velero Contributors.

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

package nodeagent

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clientTesting "k8s.io/client-go/testing"
	clientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/vmware-tanzu/velero/pkg/builder"
)

type reactor struct {
	verb        string
	resource    string
	reactorFunc clientTesting.ReactionFunc
}

func TestIsRunning(t *testing.T) {
	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-ns",
			Name:      "node-agent",
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "DaemonSet",
		},
	}

	tests := []struct {
		name          string
		kubeClientObj []runtime.Object
		namespace     string
		kubeReactors  []reactor
		expectErr     string
	}{
		{
			name:      "ds is not found",
			namespace: "fake-ns",
			expectErr: "daemonset not found",
		},
		{
			name:      "ds get error",
			namespace: "fake-ns",
			kubeReactors: []reactor{
				{
					verb:     "get",
					resource: "daemonsets",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-get-error")
					},
				},
			},
			expectErr: "fake-get-error",
		},
		{
			name:      "succeed",
			namespace: "fake-ns",
			kubeClientObj: []runtime.Object{
				daemonSet,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.kubeClientObj...)

			for _, reactor := range test.kubeReactors {
				fakeKubeClient.Fake.PrependReactor(reactor.verb, reactor.resource, reactor.reactorFunc)
			}

			err := IsRunning(context.TODO(), fakeKubeClient, test.namespace)
			if test.expectErr == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, test.expectErr)
			}
		})
	}
}

func TestIsRunningInNode(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)

	nonNodeAgentPod := builder.ForPod("fake-ns", "fake-pod").Result()
	nodeAgentPodNotRunning := builder.ForPod("fake-ns", "fake-pod").Labels(map[string]string{"name": "node-agent"}).Result()
	nodeAgentPodRunning1 := builder.ForPod("fake-ns", "fake-pod-1").Labels(map[string]string{"name": "node-agent"}).Phase(corev1.PodRunning).Result()
	nodeAgentPodRunning2 := builder.ForPod("fake-ns", "fake-pod-2").Labels(map[string]string{"name": "node-agent"}).Phase(corev1.PodRunning).Result()
	nodeAgentPodRunning3 := builder.ForPod("fake-ns", "fake-pod-3").
		Labels(map[string]string{"name": "node-agent"}).
		Phase(corev1.PodRunning).
		NodeName("fake-node").
		Result()

	tests := []struct {
		name          string
		kubeClientObj []runtime.Object
		nodeName      string
		expectErr     string
	}{
		{
			name:      "node name is empty",
			expectErr: "node name is empty",
		},
		{
			name:     "ds pod not found",
			nodeName: "fake-node",
			kubeClientObj: []runtime.Object{
				nonNodeAgentPod,
			},
			expectErr: "daemonset pod not found in running state in node fake-node",
		},
		{
			name:     "ds po are not all running",
			nodeName: "fake-node",
			kubeClientObj: []runtime.Object{
				nodeAgentPodNotRunning,
				nodeAgentPodRunning1,
			},
			expectErr: "daemonset pod not found in running state in node fake-node",
		},
		{
			name:     "ds pods wrong node name",
			nodeName: "fake-node",
			kubeClientObj: []runtime.Object{
				nodeAgentPodNotRunning,
				nodeAgentPodRunning1,
				nodeAgentPodRunning2,
			},
			expectErr: "daemonset pod not found in running state in node fake-node",
		},
		{
			name:     "succeed",
			nodeName: "fake-node",
			kubeClientObj: []runtime.Object{
				nodeAgentPodNotRunning,
				nodeAgentPodRunning1,
				nodeAgentPodRunning2,
				nodeAgentPodRunning3,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClientBuilder := clientFake.NewClientBuilder()
			fakeClientBuilder = fakeClientBuilder.WithScheme(scheme)

			fakeClient := fakeClientBuilder.WithRuntimeObjects(test.kubeClientObj...).Build()

			err := IsRunningInNode(context.TODO(), "", test.nodeName, fakeClient)
			if test.expectErr == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, test.expectErr)
			}
		})
	}
}

func TestGetPodSpec(t *testing.T) {
	podSpec := corev1.PodSpec{
		NodeName: "fake-node",
	}

	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-ns",
			Name:      "node-agent",
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "DaemonSet",
		},
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: podSpec,
			},
		},
	}

	tests := []struct {
		name          string
		kubeClientObj []runtime.Object
		namespace     string
		expectErr     string
		expectSpec    corev1.PodSpec
	}{
		{
			name:      "ds is not found",
			namespace: "fake-ns",
			expectErr: "error to get node-agent daemonset: daemonsets.apps \"node-agent\" not found",
		},
		{
			name:      "succeed",
			namespace: "fake-ns",
			kubeClientObj: []runtime.Object{
				daemonSet,
			},
			expectSpec: podSpec,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.kubeClientObj...)

			spec, err := GetPodSpec(context.TODO(), fakeKubeClient, test.namespace)
			if test.expectErr == "" {
				require.NoError(t, err)
				assert.Equal(t, *spec, test.expectSpec)
			} else {
				require.EqualError(t, err, test.expectErr)
			}
		})
	}
}

func TestGetConfigs(t *testing.T) {
	cm := builder.ForConfigMap("fake-ns", "node-agent-config").Result()
	cmWithInvalidDataFormat := builder.ForConfigMap("fake-ns", "node-agent-config").Data("fake-key", "wrong").Result()
	cmWithoutCocurrentData := builder.ForConfigMap("fake-ns", "node-agent-config").Data("fake-key", "{\"someothers\":{\"someother\": 10}}").Result()
	cmWithValidData := builder.ForConfigMap("fake-ns", "node-agent-config").Data("fake-key", "{\"loadConcurrency\":{\"globalConfig\": 5}}").Result()

	tests := []struct {
		name          string
		kubeClientObj []runtime.Object
		namespace     string
		kubeReactors  []reactor
		expectResult  *Configs
		expectErr     string
	}{
		{
			name:      "cm get error",
			namespace: "fake-ns",
			kubeReactors: []reactor{
				{
					verb:     "get",
					resource: "configmaps",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-get-error")
					},
				},
			},
			expectErr: "error to get node agent configs node-agent-config: fake-get-error",
		},
		{
			name:      "cm's data is nil",
			namespace: "fake-ns",
			kubeClientObj: []runtime.Object{
				cm,
			},
			expectErr: "data is not available in config map node-agent-config",
		},
		{
			name:      "cm's data is with invalid format",
			namespace: "fake-ns",
			kubeClientObj: []runtime.Object{
				cmWithInvalidDataFormat,
			},
			expectErr: "error to unmarshall configs from node-agent-config: invalid character 'w' looking for beginning of value",
		},
		{
			name:      "concurrency configs are not found",
			namespace: "fake-ns",
			kubeClientObj: []runtime.Object{
				cmWithoutCocurrentData,
			},
			expectResult: &Configs{},
		},
		{
			name:      "success",
			namespace: "fake-ns",
			kubeClientObj: []runtime.Object{
				cmWithValidData,
			},
			expectResult: &Configs{
				LoadConcurrency: &LoadConcurrency{
					GlobalConfig: 5,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.kubeClientObj...)

			for _, reactor := range test.kubeReactors {
				fakeKubeClient.Fake.PrependReactor(reactor.verb, reactor.resource, reactor.reactorFunc)
			}

			result, err := GetConfigs(context.TODO(), test.namespace, fakeKubeClient, "node-agent-config")
			if test.expectErr == "" {
				require.NoError(t, err)

				if test.expectResult == nil {
					assert.Nil(t, result)
				} else if test.expectResult.LoadConcurrency == nil {
					assert.Nil(t, result.LoadConcurrency)
				} else {
					assert.Equal(t, *test.expectResult.LoadConcurrency, *result.LoadConcurrency)
				}
			} else {
				require.EqualError(t, err, test.expectErr)
			}
		})
	}
}
