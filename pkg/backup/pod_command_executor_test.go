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

package backup

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	arktest "github.com/heptio/ark/pkg/util/test"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

func TestNewPodCommandExecutor(t *testing.T) {
	restClientConfig := &rest.Config{Host: "foo"}
	poster := &mockPoster{}
	pce := NewPodCommandExecutor(restClientConfig, poster).(*defaultPodCommandExecutor)
	assert.Equal(t, restClientConfig, pce.restClientConfig)
	assert.Equal(t, poster, pce.restClient)
	assert.Equal(t, &defaultStreamExecutorFactory{}, pce.streamExecutorFactory)
}

func TestExecutePodCommandMissingInputs(t *testing.T) {
	tests := []struct {
		name         string
		item         map[string]interface{}
		podNamespace string
		podName      string
		hookName     string
		hook         *v1.ExecHook
	}{
		{
			name: "missing item",
		},
		{
			name: "missing pod namespace",
			item: map[string]interface{}{},
		},
		{
			name:         "missing pod name",
			item:         map[string]interface{}{},
			podNamespace: "ns",
		},
		{
			name:         "missing hookName",
			item:         map[string]interface{}{},
			podNamespace: "ns",
			podName:      "pod",
		},
		{
			name:         "missing hook",
			item:         map[string]interface{}{},
			podNamespace: "ns",
			podName:      "pod",
			hookName:     "hook",
		},
		{
			name:         "container not found",
			item:         unstructuredOrDie(`{"kind":"Pod","spec":{"containers":[{"name":"foo"}]}}`).Object,
			podNamespace: "ns",
			podName:      "pod",
			hookName:     "hook",
			hook: &v1.ExecHook{
				Container: "missing",
			},
		},
		{
			name:         "command missing",
			item:         unstructuredOrDie(`{"kind":"Pod","spec":{"containers":[{"name":"foo"}]}}`).Object,
			podNamespace: "ns",
			podName:      "pod",
			hookName:     "hook",
			hook: &v1.ExecHook{
				Container: "foo",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := &defaultPodCommandExecutor{}
			err := e.executePodCommand(arktest.NewLogger(), test.item, test.podNamespace, test.podName, test.hookName, test.hook)
			assert.Error(t, err)
		})
	}
}

func TestExecutePodCommand(t *testing.T) {
	tests := []struct {
		name                  string
		containerName         string
		expectedContainerName string
		command               []string
		errorMode             v1.HookErrorMode
		expectedErrorMode     v1.HookErrorMode
		timeout               time.Duration
		expectedTimeout       time.Duration
		hookError             error
		expectedError         string
	}{
		{
			name:                  "validate defaults",
			command:               []string{"some", "command"},
			expectedContainerName: "foo",
			expectedErrorMode:     v1.HookErrorModeFail,
			expectedTimeout:       30 * time.Second,
		},
		{
			name:                  "use specified values",
			command:               []string{"some", "command"},
			containerName:         "bar",
			expectedContainerName: "bar",
			errorMode:             v1.HookErrorModeContinue,
			expectedErrorMode:     v1.HookErrorModeContinue,
			timeout:               10 * time.Second,
			expectedTimeout:       10 * time.Second,
		},
		{
			name:                  "hook error",
			command:               []string{"some", "command"},
			expectedContainerName: "foo",
			expectedErrorMode:     v1.HookErrorModeFail,
			expectedTimeout:       30 * time.Second,
			hookError:             errors.New("hook error"),
			expectedError:         "hook error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hook := v1.ExecHook{
				Container: test.containerName,
				Command:   test.command,
				OnError:   test.errorMode,
				Timeout:   metav1.Duration{Duration: test.timeout},
			}

			pod, err := getAsMap(`
{
	"metadata": {
		"namespace": "namespace",
		"name": "name"
	},
	"spec": {
		"containers": [
			{"name": "foo"},
			{"name": "bar"}
		]
	}
}`)

			require.NoError(t, err)

			clientConfig := &rest.Config{}
			poster := &mockPoster{}
			defer poster.AssertExpectations(t)
			podCommandExecutor := NewPodCommandExecutor(clientConfig, poster).(*defaultPodCommandExecutor)

			streamExecutorFactory := &mockStreamExecutorFactory{}
			defer streamExecutorFactory.AssertExpectations(t)
			podCommandExecutor.streamExecutorFactory = streamExecutorFactory

			baseUrl, _ := url.Parse("https://some.server")
			contentConfig := rest.ContentConfig{
				GroupVersion: &schema.GroupVersion{Group: "", Version: "v1"},
			}
			postRequest := rest.NewRequest(nil, "POST", baseUrl, "/api/v1", contentConfig, rest.Serializers{}, nil, nil)
			poster.On("Post").Return(postRequest)

			streamExecutor := &mockStreamExecutor{}
			defer streamExecutor.AssertExpectations(t)

			expectedCommand := strings.Join(test.command, "&command=")
			expectedURL, _ := url.Parse(
				fmt.Sprintf("https://some.server/api/v1/namespaces/namespace/pods/name/exec?command=%s&container=%s&stderr=true&stdout=true", expectedCommand, test.expectedContainerName),
			)
			streamExecutorFactory.On("NewSPDYExecutor", clientConfig, "POST", expectedURL).Return(streamExecutor, nil)

			var stdout, stderr bytes.Buffer
			expectedStreamOptions := remotecommand.StreamOptions{
				Stdout: &stdout,
				Stderr: &stderr,
			}
			streamExecutor.On("Stream", expectedStreamOptions).Return(test.hookError)

			err = podCommandExecutor.executePodCommand(arktest.NewLogger(), pod, "namespace", "name", "hookName", &hook)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestEnsureContainerExists(t *testing.T) {
	pod := map[string]interface{}{
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name": "foo",
				},
			},
		},
	}

	err := ensureContainerExists(pod, "bar")
	assert.EqualError(t, err, `no such container: "bar"`)

	err = ensureContainerExists(pod, "foo")
	assert.NoError(t, err)
}

type mockStreamExecutorFactory struct {
	mock.Mock
}

func (f *mockStreamExecutorFactory) NewSPDYExecutor(config *rest.Config, method string, url *url.URL) (remotecommand.Executor, error) {
	args := f.Called(config, method, url)
	return args.Get(0).(remotecommand.Executor), args.Error(1)
}

type mockStreamExecutor struct {
	mock.Mock
	remotecommand.Executor
}

func (e *mockStreamExecutor) Stream(options remotecommand.StreamOptions) error {
	args := e.Called(options)
	return args.Error(0)
}

type mockPoster struct {
	mock.Mock
}

func (p *mockPoster) Post() *rest.Request {
	args := p.Called()
	return args.Get(0).(*rest.Request)
}

type mockPodCommandExecutor struct {
	mock.Mock
}

func (e *mockPodCommandExecutor) executePodCommand(log *logrus.Entry, item map[string]interface{}, namespace, name, hookName string, hook *v1.ExecHook) error {
	args := e.Called(log, item, namespace, name, hookName, hook)
	return args.Error(0)
}
