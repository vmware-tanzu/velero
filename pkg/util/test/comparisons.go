/*
Copyright 2018 the Heptio Ark contributors.

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
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	core "k8s.io/client-go/testing"
)

// CompareActions checks slices of actual and expected Actions
// for equality (ignoring order). It checks that the lengths of
// the slices are the same, that each actual Action has a
// corresponding expected Action, and that each expected Action
// has a corresponding actual Action.
func CompareActions(t *testing.T, expected, actual []core.Action) {
	assert.Len(t, actual, len(expected))

	for _, e := range expected {
		found := false
		for _, a := range actual {
			if reflect.DeepEqual(e, a) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing expected action %#v", e)
		}
	}

	for _, a := range actual {
		found := false
		for _, e := range expected {
			if reflect.DeepEqual(e, a) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("unexpected action %#v", a)
		}
	}
}
