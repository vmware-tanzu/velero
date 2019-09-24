/*
Copyright 2018 the Velero contributors.

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
package clientmgmt

import (
	"reflect"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
)

type restartableDelegateTest struct {
	function                string
	inputs                  []interface{}
	expectedErrorOutputs    []interface{}
	expectedDelegateOutputs []interface{}
}

type mockable interface {
	Test(t mock.TestingT)
	On(method string, args ...interface{}) *mock.Call
	AssertExpectations(t mock.TestingT) bool
}

func runRestartableDelegateTests(
	t *testing.T,
	kind framework.PluginKind,
	newRestartable func(key kindAndName, p RestartableProcess) interface{},
	newMock func() mockable,
	tests ...restartableDelegateTest,
) {
	for _, tc := range tests {
		t.Run(tc.function, func(t *testing.T) {
			p := new(mockRestartableProcess)
			p.Test(t)
			defer p.AssertExpectations(t)

			// getDelegate error
			p.On("resetIfNeeded").Return(errors.Errorf("reset error")).Once()
			name := "delegateName"
			key := kindAndName{kind: kind, name: name}
			r := newRestartable(key, p)

			// Get the method we're going to call using reflection
			method := reflect.ValueOf(r).MethodByName(tc.function)
			require.NotEmpty(t, method)

			// Convert the test case inputs ([]interface{}) to []reflect.Value
			var inputValues []reflect.Value
			for i := range tc.inputs {
				inputValues = append(inputValues, reflect.ValueOf(tc.inputs[i]))
			}

			// Invoke the method being tested
			actual := method.Call(inputValues)

			// This function asserts that the actual outputs match the expected outputs
			checkOutputs := func(expected []interface{}, actual []reflect.Value) {
				require.Equal(t, len(expected), len(actual))

				for i := range actual {
					// Get the underlying value from the reflect.Value
					a := actual[i].Interface()

					// Check if it's an error
					actualErr, actualErrOk := a.(error)
					// Check if the expected output element is an error
					expectedErr, expectedErrOk := expected[i].(error)
					// If both are errors, use EqualError
					if actualErrOk && expectedErrOk {
						assert.EqualError(t, actualErr, expectedErr.Error())
						continue
					}

					// If function returns nil as struct return type, we cannot just
					// compare the interface to nil as its type will not be nil,
					// only the value will be
					if expected[i] == nil && reflect.ValueOf(a).Kind() == reflect.Ptr {
						assert.True(t, reflect.ValueOf(a).IsNil())
						continue
					}

					// Otherwise, use plain Equal
					assert.Equal(t, expected[i], a)
				}
			}

			// Make sure we get what we expected when getDelegate returned an error
			checkOutputs(tc.expectedErrorOutputs, actual)

			// Invoke delegate, make sure all returned values are passed through
			p.On("resetIfNeeded").Return(nil)

			delegate := newMock()
			delegate.Test(t)
			defer delegate.AssertExpectations(t)

			p.On("getByKindAndName", key).Return(delegate, nil)

			// Set up the mocked method in the delegate
			delegate.On(tc.function, tc.inputs...).Return(tc.expectedDelegateOutputs...)

			// Invoke the method being tested
			actual = method.Call(inputValues)

			// Make sure we get what we expected when invoking the delegate
			checkOutputs(tc.expectedDelegateOutputs, actual)
		})
	}
}
