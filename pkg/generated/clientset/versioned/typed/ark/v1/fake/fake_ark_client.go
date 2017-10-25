/*
Copyright 2017 the Heptio Ark contributors.

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
package fake

import (
	v1 "github.com/heptio/ark/pkg/generated/clientset/versioned/typed/ark/v1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeArkV1 struct {
	*testing.Fake
}

func (c *FakeArkV1) Backups(namespace string) v1.BackupInterface {
	return &FakeBackups{c, namespace}
}

func (c *FakeArkV1) Configs(namespace string) v1.ConfigInterface {
	return &FakeConfigs{c, namespace}
}

func (c *FakeArkV1) DownloadRequests(namespace string) v1.DownloadRequestInterface {
	return &FakeDownloadRequests{c, namespace}
}

func (c *FakeArkV1) Restores(namespace string) v1.RestoreInterface {
	return &FakeRestores{c, namespace}
}

func (c *FakeArkV1) Schedules(namespace string) v1.ScheduleInterface {
	return &FakeSchedules{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeArkV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
