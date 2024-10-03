/*
Copyright the Velero contributors.

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

package backend

import (
	"context"

	"github.com/kopia/kopia/repo/blob"

	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo"
	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo/kopialib/backend/azure"
)

type AzureBackend struct {
	option azure.Option
}

func (c *AzureBackend) Setup(ctx context.Context, flags map[string]string) error {
	if flags[udmrepo.StoreOptionCACert] != "" {
		flags["caCertEncoded"] = "true"
	}
	c.option = azure.Option{
		Config: flags,
		Limits: setupLimits(ctx, flags),
	}
	return nil
}

func (c *AzureBackend) Connect(ctx context.Context, isCreate bool) (blob.Storage, error) {
	return azure.NewStorage(ctx, &c.option, false)
}
