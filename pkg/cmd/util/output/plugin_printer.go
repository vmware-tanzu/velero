/*
Copyright 2017, 2019 the Velero contributors.

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
	sortPluginsByName(list)

	for _, plugin := range list.Status.Plugins {
		if err := printPlugin(plugin, w, options); err != nil {
			return err
		}
	}
	return nil
}

func sortPluginsByName(list *velerov1api.ServerStatusRequest) {
	sort.Slice(list.Status.Plugins, func(i, j int) bool {
		return list.Status.Plugins[i][0] < list.Status.Plugins[j][0]
	})
}

func printPlugin(plugin []string, w io.Writer, options printers.PrintOptions) error {
	pluginName := plugin[0]
	pluginKind := plugin[1]
	name := printers.FormatResourceName(options.Kind, pluginName, options.WithKind)

	if _, err := fmt.Fprintf(w, "%s\t%s\n", name, pluginKind); err != nil {
		return err
	}

	return nil
}
