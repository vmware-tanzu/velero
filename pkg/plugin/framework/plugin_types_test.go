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

package framework

import (
	"testing"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/stretchr/testify/assert"
)

func TestPluginImplementationsAreGRPCPlugins(t *testing.T) {
	pluginImpls := []interface{}{
		new(VolumeSnapshotterPlugin),
		new(BackupItemActionPlugin),
		new(ObjectStorePlugin),
		new(PluginListerPlugin),
		new(RestoreItemActionPlugin),
	}

	for _, impl := range pluginImpls {
		_, ok := impl.(plugin.GRPCPlugin)
		assert.True(t, ok, "plugin implementation %T does not implement the go-plugin.GRPCPlugin interface", impl)
	}
}
