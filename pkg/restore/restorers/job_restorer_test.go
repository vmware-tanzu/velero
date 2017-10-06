/*
Copyright 2017 Heptio Inc.

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

package restorers

import (
	"testing"

	testlogger "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestJobRestorerPrepare(t *testing.T) {
	tests := []struct {
		name        string
		obj         runtime.Unstructured
		expectedErr bool
		expectedRes runtime.Unstructured
	}{
		{
			name:        "no metadata should error",
			obj:         NewTestUnstructured().Unstructured,
			expectedErr: true,
		},
		{
			name: "missing spec.selector and/or spec.template should not error",
			obj: NewTestUnstructured().WithName("job-1").
				WithSpec().
				Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("job-1").
				WithSpec().
				Unstructured,
		},
		{
			name: "missing spec.selector.matchLabels should not error",
			obj: NewTestUnstructured().WithName("job-1").
				WithSpecField("selector", map[string]interface{}{}).
				Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("job-1").
				WithSpecField("selector", map[string]interface{}{}).
				Unstructured,
		},
		{
			name: "spec.selector.matchLabels[controller-uid] is removed",
			obj: NewTestUnstructured().WithName("job-1").
				WithSpecField("selector", map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"controller-uid": "foo",
						"hello":          "world",
					},
				}).
				Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("job-1").
				WithSpecField("selector", map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"hello": "world",
					},
				}).
				Unstructured,
		},
		{
			name: "missing spec.template.metadata should not error",
			obj: NewTestUnstructured().WithName("job-1").
				WithSpecField("template", map[string]interface{}{}).
				Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("job-1").
				WithSpecField("template", map[string]interface{}{}).
				Unstructured,
		},
		{
			name: "missing spec.template.metadata.labels should not error",
			obj: NewTestUnstructured().WithName("job-1").
				WithSpecField("template", map[string]interface{}{
					"metadata": map[string]interface{}{},
				}).
				Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("job-1").
				WithSpecField("template", map[string]interface{}{
					"metadata": map[string]interface{}{},
				}).
				Unstructured,
		},
		{
			name: "spec.template.metadata.labels[controller-uid] is removed",
			obj: NewTestUnstructured().WithName("job-1").
				WithSpecField("template", map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"controller-uid": "foo",
							"hello":          "world",
						},
					},
				}).
				Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("job-1").
				WithSpecField("template", map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"hello": "world",
						},
					},
				}).
				Unstructured,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				logger, _ = testlogger.NewNullLogger()
				restorer  = NewJobRestorer(logger)
			)

			res, _, err := restorer.Prepare(test.obj, nil, nil)

			if assert.Equal(t, test.expectedErr, err != nil) {
				assert.Equal(t, test.expectedRes, res)
			}
		})
	}
}
