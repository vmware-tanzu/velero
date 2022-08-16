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
	"github.com/kopia/kopia/repo/blob/azure"

	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo"
)

type AzureBackend struct {
	options azure.Options
}

func (c *AzureBackend) Setup(ctx context.Context, flags map[string]string) error {
	var err error
	c.options.Container, err = mustHaveString(udmrepo.StoreOptionOssBucket, flags)
	if err != nil {
		return err
	}

	c.options.StorageAccount, err = mustHaveString(udmrepo.StoreOptionAzureStorageAccount, flags)
	if err != nil {
		return err
	}

	c.options.StorageKey, err = mustHaveString(udmrepo.StoreOptionAzureKey, flags)
	if err != nil {
		return err
	}

	c.options.Prefix = optionalHaveString(udmrepo.StoreOptionPrefix, flags)
	c.options.SASToken = optionalHaveString(udmrepo.StoreOptionAzureToken, flags)
	c.options.StorageDomain = optionalHaveString(udmrepo.StoreOptionAzureDomain, flags)

	c.options.Limits = setupLimits(ctx, flags)

	return nil
}

func (c *AzureBackend) Connect(ctx context.Context, isCreate bool) (blob.Storage, error) {
	return azure.New(ctx, &c.options)
}
