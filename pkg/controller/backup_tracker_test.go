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

package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBackupTracker(t *testing.T) {
	bt := NewBackupTracker()

	assert.False(t, bt.Contains("ns", "name"))

	bt.Add("ns", "name")
	assert.True(t, bt.Contains("ns", "name"))

	bt.Add("ns2", "name2")
	assert.True(t, bt.Contains("ns", "name"))
	assert.True(t, bt.Contains("ns2", "name2"))

	bt.Delete("ns", "name")
	assert.False(t, bt.Contains("ns", "name"))
	assert.True(t, bt.Contains("ns2", "name2"))

	bt.Delete("ns2", "name2")
	assert.False(t, bt.Contains("ns2", "name2"))
}
