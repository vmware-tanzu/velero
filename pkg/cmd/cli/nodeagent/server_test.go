/*
Copyright 2019 the Velero contributors.

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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/nodeagent"
	testutil "github.com/vmware-tanzu/velero/pkg/test"
)

func Test_validatePodVolumesHostPath(t *testing.T) {
	tests := []struct {
		name    string
		pods    []*corev1.Pod
		dirs    []string
		wantErr bool
	}{
		{
			name: "no error when pod volumes are present",
			pods: []*corev1.Pod{
				builder.ForPod("foo", "bar").ObjectMeta(builder.WithUID("foo")).Result(),
				builder.ForPod("zoo", "raz").ObjectMeta(builder.WithUID("zoo")).Result(),
			},
			dirs:    []string{"foo", "zoo"},
			wantErr: false,
		},
		{
			name: "no error when pod volumes are present and there are mirror pods",
			pods: []*corev1.Pod{
				builder.ForPod("foo", "bar").ObjectMeta(builder.WithUID("foo")).Result(),
				builder.ForPod("zoo", "raz").ObjectMeta(builder.WithUID("zoo"), builder.WithAnnotations(corev1.MirrorPodAnnotationKey, "baz")).Result(),
			},
			dirs:    []string{"foo", "baz"},
			wantErr: false,
		},
		{
			name: "error when all pod volumes missing",
			pods: []*corev1.Pod{
				builder.ForPod("foo", "bar").ObjectMeta(builder.WithUID("foo")).Result(),
				builder.ForPod("zoo", "raz").ObjectMeta(builder.WithUID("zoo")).Result(),
			},
			dirs:    []string{"unexpected-dir"},
			wantErr: true,
		},
		{
			name: "error when some pod volumes missing",
			pods: []*corev1.Pod{
				builder.ForPod("foo", "bar").ObjectMeta(builder.WithUID("foo")).Result(),
				builder.ForPod("zoo", "raz").ObjectMeta(builder.WithUID("zoo")).Result(),
			},
			dirs:    []string{"foo"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := testutil.NewFakeFileSystem()

			for _, dir := range tt.dirs {
				err := fs.MkdirAll(filepath.Join("/host_pods/", dir), os.ModePerm)
				if err != nil {
					t.Error(err)
				}
			}

			kubeClient := fake.NewSimpleClientset()
			for _, pod := range tt.pods {
				_, err := kubeClient.CoreV1().Pods(pod.GetNamespace()).Create(context.TODO(), pod, metav1.CreateOptions{})
				if err != nil {
					t.Error(err)
				}
			}

			s := &nodeAgentServer{
				logger:     testutil.NewLogger(),
				fileSystem: fs,
			}

			err := s.validatePodVolumesHostPath(kubeClient)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_getDataPathConfigs(t *testing.T) {
	configs := &nodeagent.Configs{
		LoadConcurrency: &nodeagent.LoadConcurrency{
			GlobalConfig: -1,
		},
	}

	tests := []struct {
		name          string
		getFunc       func(context.Context, string, kubernetes.Interface, string) (*nodeagent.Configs, error)
		configMapName string
		expectConfigs *nodeagent.Configs
		expectLog     string
	}{
		{
			name:      "no config specified",
			expectLog: "No node-agent configMap is specified",
		},
		{
			name:          "failed to get configs",
			configMapName: "node-agent-config",
			getFunc: func(context.Context, string, kubernetes.Interface, string) (*nodeagent.Configs, error) {
				return nil, errors.New("fake-get-error")
			},
			expectLog: "Failed to get node agent configs from configMap node-agent-config, ignore it",
		},
		{
			name:          "configs cm not found",
			configMapName: "node-agent-config",
			getFunc: func(context.Context, string, kubernetes.Interface, string) (*nodeagent.Configs, error) {
				return nil, errors.New("fake-not-found-error")
			},
			expectLog: "Failed to get node agent configs from configMap node-agent-config, ignore it",
		},

		{
			name:          "succeed",
			configMapName: "node-agent-config",
			getFunc: func(context.Context, string, kubernetes.Interface, string) (*nodeagent.Configs, error) {
				return configs, nil
			},
			expectConfigs: configs,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logBuffer := ""

			s := &nodeAgentServer{
				config: nodeAgentServerConfig{
					nodeAgentConfig: test.configMapName,
				},
				logger: testutil.NewSingleLogger(&logBuffer),
			}

			getConfigsFunc = test.getFunc

			s.getDataPathConfigs()
			assert.Equal(t, test.expectConfigs, s.dataPathConfigs)
			if test.expectLog == "" {
				assert.Equal(t, "", logBuffer)
			} else {
				assert.Contains(t, logBuffer, test.expectLog)
			}
		})
	}
}

func Test_getDataPathConcurrentNum(t *testing.T) {
	defaultNum := 100001
	globalNum := 6
	nodeName := "node-agent-node"
	node1 := builder.ForNode("node-agent-node").Result()
	node2 := builder.ForNode("node-agent-node").Labels(map[string]string{
		"host-name": "node-1",
		"xxxx":      "yyyyy",
	}).Result()

	invalidLabelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"inva/lid": "inva/lid",
		},
	}
	validLabelSelector1 := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"host-name": "node-1",
		},
	}
	validLabelSelector2 := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"xxxx": "yyyyy",
		},
	}

	tests := []struct {
		name          string
		configs       nodeagent.Configs
		setKubeClient bool
		kubeClientObj []runtime.Object
		expectNum     int
		expectLog     string
	}{
		{
			name:      "configs cm's data path concurrency is nil",
			expectLog: fmt.Sprintf("Concurrency configs are not found, use the default number %v", defaultNum),
			expectNum: defaultNum,
		},
		{
			name: "global number is invalid",
			configs: nodeagent.Configs{
				LoadConcurrency: &nodeagent.LoadConcurrency{
					GlobalConfig: -1,
				},
			},
			expectLog: fmt.Sprintf("Global number %v is invalid, use the default value %v", -1, defaultNum),
			expectNum: defaultNum,
		},
		{
			name: "global number is valid",
			configs: nodeagent.Configs{
				LoadConcurrency: &nodeagent.LoadConcurrency{
					GlobalConfig: globalNum,
				},
			},
			expectNum: globalNum,
		},
		{
			name: "node is not found",
			configs: nodeagent.Configs{
				LoadConcurrency: &nodeagent.LoadConcurrency{
					GlobalConfig: globalNum,
					PerNodeConfig: []nodeagent.RuledConfigs{
						{
							Number: 100,
						},
					},
				},
			},
			setKubeClient: true,
			expectLog:     fmt.Sprintf("Failed to get node info for %s, use the global number %v", nodeName, globalNum),
			expectNum:     globalNum,
		},
		{
			name: "failed to get selector",
			configs: nodeagent.Configs{
				LoadConcurrency: &nodeagent.LoadConcurrency{
					GlobalConfig: globalNum,
					PerNodeConfig: []nodeagent.RuledConfigs{
						{
							NodeSelector: invalidLabelSelector,
							Number:       100,
						},
					},
				},
			},
			setKubeClient: true,
			kubeClientObj: []runtime.Object{node1},
			expectLog:     fmt.Sprintf("Failed to parse rule with label selector %s, skip it", invalidLabelSelector.String()),
			expectNum:     globalNum,
		},
		{
			name: "rule number is invalid",
			configs: nodeagent.Configs{
				LoadConcurrency: &nodeagent.LoadConcurrency{
					GlobalConfig: globalNum,
					PerNodeConfig: []nodeagent.RuledConfigs{
						{
							NodeSelector: validLabelSelector1,
							Number:       -1,
						},
					},
				},
			},
			setKubeClient: true,
			kubeClientObj: []runtime.Object{node1},
			expectLog:     fmt.Sprintf("Rule with label selector %s is with an invalid number %v, skip it", validLabelSelector1.String(), -1),
			expectNum:     globalNum,
		},
		{
			name: "label doesn't match",
			configs: nodeagent.Configs{
				LoadConcurrency: &nodeagent.LoadConcurrency{
					GlobalConfig: globalNum,
					PerNodeConfig: []nodeagent.RuledConfigs{
						{
							NodeSelector: validLabelSelector1,
							Number:       -1,
						},
					},
				},
			},
			setKubeClient: true,
			kubeClientObj: []runtime.Object{node1},
			expectLog:     fmt.Sprintf("Per node number for node %s is not found, use the global number %v", nodeName, globalNum),
			expectNum:     globalNum,
		},
		{
			name: "match one rule",
			configs: nodeagent.Configs{
				LoadConcurrency: &nodeagent.LoadConcurrency{
					GlobalConfig: globalNum,
					PerNodeConfig: []nodeagent.RuledConfigs{
						{
							NodeSelector: validLabelSelector1,
							Number:       66,
						},
					},
				},
			},
			setKubeClient: true,
			kubeClientObj: []runtime.Object{node2},
			expectLog:     fmt.Sprintf("Use the per node number %v over global number %v for node %s", 66, globalNum, nodeName),
			expectNum:     66,
		},
		{
			name: "match multiple rules",
			configs: nodeagent.Configs{
				LoadConcurrency: &nodeagent.LoadConcurrency{
					GlobalConfig: globalNum,
					PerNodeConfig: []nodeagent.RuledConfigs{
						{
							NodeSelector: validLabelSelector1,
							Number:       66,
						},
						{
							NodeSelector: validLabelSelector2,
							Number:       36,
						},
					},
				},
			},
			setKubeClient: true,
			kubeClientObj: []runtime.Object{node2},
			expectLog:     fmt.Sprintf("Use the per node number %v over global number %v for node %s", 36, globalNum, nodeName),
			expectNum:     36,
		},
		{
			name: "match multiple rules 2",
			configs: nodeagent.Configs{
				LoadConcurrency: &nodeagent.LoadConcurrency{
					GlobalConfig: globalNum,
					PerNodeConfig: []nodeagent.RuledConfigs{
						{
							NodeSelector: validLabelSelector1,
							Number:       36,
						},
						{
							NodeSelector: validLabelSelector2,
							Number:       66,
						},
					},
				},
			},
			setKubeClient: true,
			kubeClientObj: []runtime.Object{node2},
			expectLog:     fmt.Sprintf("Use the per node number %v over global number %v for node %s", 36, globalNum, nodeName),
			expectNum:     36,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.kubeClientObj...)

			logBuffer := ""

			s := &nodeAgentServer{
				nodeName:        nodeName,
				dataPathConfigs: &test.configs,
				logger:          testutil.NewSingleLogger(&logBuffer),
			}

			if test.setKubeClient {
				s.kubeClient = fakeKubeClient
			}

			num := s.getDataPathConcurrentNum(defaultNum)
			assert.Equal(t, test.expectNum, num)
			if test.expectLog == "" {
				assert.Equal(t, "", logBuffer)
			} else {
				assert.Contains(t, logBuffer, test.expectLog)
			}
		})
	}
}
