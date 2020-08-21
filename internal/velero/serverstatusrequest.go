/*
Copyright 2020 the Velero contributors.

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

package velero

import (
	"context"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/buildinfo"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
)

// ServerStatus holds information for retrieving installed
// plugins and for updating the ServerStatusRequest timestamp.
type ServerStatus struct {
	PluginRegistry PluginLister
	Clock          clock.Clock
}

// PatchStatusProcessed patches status fields, including loading the plugin info, and updates
// the ServerStatusRequest.Status.Phase to ServerStatusRequestPhaseProcessed.
func (s *ServerStatus) PatchStatusProcessed(kbClient client.Client, req *velerov1api.ServerStatusRequest, ctx context.Context) error {
	statusPatch := client.MergeFrom(req.DeepCopyObject())
	req.Status.ServerVersion = buildinfo.Version
	req.Status.Phase = velerov1api.ServerStatusRequestPhaseProcessed
	req.Status.ProcessedTimestamp = &metav1.Time{Time: s.Clock.Now()}
	req.Status.Plugins = getInstalledPluginInfo(s.PluginRegistry)

	if err := kbClient.Status().Patch(ctx, req, statusPatch); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

type PluginLister interface {
	// List returns all PluginIdentifiers for kind.
	List(kind framework.PluginKind) []framework.PluginIdentifier
}

func getInstalledPluginInfo(pluginLister PluginLister) []velerov1api.PluginInfo {
	var plugins []velerov1api.PluginInfo
	for _, v := range framework.AllPluginKinds() {
		list := pluginLister.List(v)
		for _, plugin := range list {
			pluginInfo := velerov1api.PluginInfo{
				Name: plugin.Name,
				Kind: plugin.Kind.String(),
			}
			plugins = append(plugins, pluginInfo)
		}
	}
	return plugins
}
