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

package output

import (
	"fmt"
	"io"
	"sort"

	"k8s.io/kubernetes/pkg/printers"

	velerov1api "github.com/heptio/velero/pkg/apis/velero/v1"
)

var (
	pluginColumns = []string{"NAME", "KIND"}
)

func printPluginList(list *velerov1api.ServerStatusRequest, w io.Writer, options printers.PrintOptions) error {
	plugins := list.Status.Plugins
	sortByKindAndName(plugins)

	for _, plugin := range plugins {
		if err := printPlugin(plugin, w, options); err != nil {
			return err
		}
	}
	return nil
}

func sortByKindAndName(plugins []velerov1api.PluginInfo) {
	sort.Slice(plugins, func(i, j int) bool {
		if plugins[i].Kind != plugins[j].Kind {
			return plugins[i].Kind < plugins[j].Kind
		}
		return plugins[i].Name < plugins[j].Name
	})
}

func printPlugin(plugin velerov1api.PluginInfo, w io.Writer, options printers.PrintOptions) error {
	name := printers.FormatResourceName(options.Kind, plugin.Name, options.WithKind)

	if _, err := fmt.Fprintf(w, "%s\t%s\n", name, plugin.Kind); err != nil {
		return err
	}

	return nil
}
