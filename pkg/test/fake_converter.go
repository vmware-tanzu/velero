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

package test

import (
	"github.com/stretchr/testify/mock"
)

type FakeUnstructuredConverter struct {
	mock.Mock
}

func (m *FakeUnstructuredConverter) FromUnstructured(u map[string]any, obj any) error {
	args := m.Called(u, obj)
	return args.Error(0)
}

func (m *FakeUnstructuredConverter) ToUnstructured(obj any) (map[string]any, error) {
	args := m.Called(obj)
	return args.Get(0).(map[string]any), args.Error(1)
}
