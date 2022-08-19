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
	"github.com/kopia/kopia/repo/blob/s3"

	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo"
)

type S3Backend struct {
	options s3.Options
}

func (c *S3Backend) Setup(ctx context.Context, flags map[string]string) error {
	var err error
	c.options.BucketName, err = mustHaveString(udmrepo.StoreOptionOssBucket, flags)
	if err != nil {
		return err
	}

	c.options.AccessKeyID, err = mustHaveString(udmrepo.StoreOptionS3KeyId, flags)
	if err != nil {
		return err
	}

	c.options.SecretAccessKey, err = mustHaveString(udmrepo.StoreOptionS3SecretKey, flags)
	if err != nil {
		return err
	}

	c.options.Endpoint = optionalHaveString(udmrepo.StoreOptionS3Endpoint, flags)
	c.options.Region = optionalHaveString(udmrepo.StoreOptionOssRegion, flags)
	c.options.Prefix = optionalHaveString(udmrepo.StoreOptionPrefix, flags)
	c.options.DoNotUseTLS = optionalHaveBool(ctx, udmrepo.StoreOptionS3DisableTls, flags)
	c.options.DoNotVerifyTLS = optionalHaveBool(ctx, udmrepo.StoreOptionS3DisableTlsVerify, flags)
	c.options.SessionToken = optionalHaveString(udmrepo.StoreOptionS3Token, flags)

	c.options.Limits = setupLimits(ctx, flags)

	return nil
}

func (c *S3Backend) Connect(ctx context.Context, isCreate bool) (blob.Storage, error) {
	return s3.New(ctx, &c.options)
}
