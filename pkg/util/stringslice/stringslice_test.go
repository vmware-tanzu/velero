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

package stringslice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHas(t *testing.T) {
	items := []string{}
	assert.False(t, Has(items, "a"))

	items = []string{"a", "b", "c"}
	assert.True(t, Has(items, "a"))
	assert.True(t, Has(items, "b"))
	assert.True(t, Has(items, "c"))
	assert.False(t, Has(items, "d"))
}

func TestExcept(t *testing.T) {
	items := []string{}
	except := Except(items, "asdf")
	assert.Len(t, except, 0)

	items = []string{"a", "b", "c"}
	assert.Equal(t, []string{"b", "c"}, Except(items, "a"))
	assert.Equal(t, []string{"a", "c"}, Except(items, "b"))
	assert.Equal(t, []string{"a", "b"}, Except(items, "c"))
	assert.Equal(t, []string{"a", "b", "c"}, Except(items, "d"))
}
