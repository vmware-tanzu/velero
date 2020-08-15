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
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/buildinfo"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
)

const ttl = time.Minute

type PluginLister interface {
	// List returns all PluginIdentifiers for kind.
	List(kind framework.PluginKind) []framework.PluginIdentifier
}

// Process fills out new ServerStatusRequest objects and deletes processed ones
// that have expired.
func Process(req *velerov1api.ServerStatusRequest, kbClient kbclient.Client, pluginLister PluginLister, clock clock.Clock, log logrus.FieldLogger) error {
	switch req.Status.Phase {
	case "", velerov1api.ServerStatusRequestPhaseNew:
		log.Info("Processing yyy new ServerStatusRequest")
		log.Info("--->>> buildinfo.Version is...: ", buildinfo.Version)
		return errors.WithStack(patch(kbClient, req, func(req *velerov1api.ServerStatusRequest) {
			req.Status.ServerVersion = buildinfo.Version
			req.Status.ProcessedTimestamp = &metav1.Time{Time: clock.Now()}
			req.Status.Phase = velerov1api.ServerStatusRequestPhaseProcessed
			req.Status.Plugins = plugins(pluginLister)
		}))
	case velerov1api.ServerStatusRequestPhaseProcessed:
		log.Debug("Checking whether ServerStatusRequest has expired")
		expiration := req.Status.ProcessedTimestamp.Add(ttl)
		if expiration.After(clock.Now()) {
			log.Debug("ServerStatusRequest has not expired")
			return nil
		}

		log.Debug("ServerStatusRequest has expired, deleting it")
		if err := kbClient.Delete(context.TODO(), req); err != nil {
			return errors.WithStack(err)
		}

		return nil
	default:
		return errors.Errorf("unexpected ServerStatusRequest phase %q", req.Status.Phase)
	}
}

func patch(kbClient client.Client, req *velerov1api.ServerStatusRequest, updateFunc func(*velerov1api.ServerStatusRequest)) error {
	patch := client.MergeFrom(req.DeepCopyObject())

	updateFunc(req)

	if err := kbClient.Status().Patch(context.TODO(), req, patch); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func plugins(pluginLister PluginLister) []velerov1api.PluginInfo {
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
