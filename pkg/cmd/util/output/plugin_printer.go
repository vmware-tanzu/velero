/*
Copyright 2019, 2020 the Velero contributors.

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
	"encoding/json"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/cmd/server/plugin"
)

var (
	pluginColumns = []metav1.TableColumnDefinition{
		// name needs Type and Format defined for the decorator to identify it:
		// https://github.com/kubernetes/kubernetes/blob/v1.15.3/pkg/printers/tableprinter.go#L204
		{Name: "Name", Type: "string", Format: "name"},
		{Name: "Kind"},
		{Name: "System-Default-Plugin"},
	}
)

type pluginInfo struct {
	Name                string `json:"name"`
	Kind                string `json:"kind"`
	SystemDefaultPlugin bool   `json:"system_default_plugin"`
}

func printPluginList(list *velerov1api.ServerStatusRequest) []metav1.TableRow {
	plugins := list.Status.Plugins
	sortByKindAndName(plugins)

	rows := make([]metav1.TableRow, 0, len(plugins))

	for _, p := range plugins {
		rows = append(rows, printPlugin(p)...)
	}
	return rows
}

func sortByKindAndName(plugins []velerov1api.PluginInfo) {
	sort.Slice(plugins, func(i, j int) bool {
		if plugins[i].Kind != plugins[j].Kind {
			return plugins[i].Kind < plugins[j].Kind
		}
		return plugins[i].Name < plugins[j].Name
	})
}

func printPlugin(plugin velerov1api.PluginInfo) []metav1.TableRow {
	row := metav1.TableRow{}

	row.Cells = append(row.Cells, plugin.Name, plugin.Kind, isDefaultPlugin(plugin.Name))

	return []metav1.TableRow{row}
}

func isDefaultPlugin(name string) bool {
	for _, p := range plugin.DefaultPlugins {
		if strings.ToLower(p) == strings.ToLower(name) {
			return true
		}
	}
	return false
}

func printPluginEncoded(list *velerov1api.ServerStatusRequest, format string) ([]byte, error) {
	var info []pluginInfo

	for _, p := range list.Status.Plugins {
		info = append(info, pluginInfo{
			Name:                p.Name,
			Kind:                p.Kind,
			SystemDefaultPlugin: isDefaultPlugin(p.Name),
		})
	}

	switch format {

	case "yaml":
		res, err := yaml.Marshal(info)
		if err != nil {
			return []byte{}, err
		}
		return res, err

	case "json":
		res, err := json.MarshalIndent(info, "", "\t")
		if err != nil {
			return []byte{}, err
		}
		return res, err

	default:
		return []byte{}, errors.Errorf("format %s is not supported", format)
	}
}
