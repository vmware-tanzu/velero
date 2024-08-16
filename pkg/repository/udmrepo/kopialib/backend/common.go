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
	"os"
	"slices"
	"time"

	"github.com/kopia/kopia/repo"
	"github.com/kopia/kopia/repo/blob"
	"github.com/kopia/kopia/repo/blob/throttling"
	"github.com/kopia/kopia/repo/content"
	"github.com/kopia/kopia/repo/encryption"
	"github.com/kopia/kopia/repo/format"
	"github.com/kopia/kopia/repo/hashing"
	"github.com/kopia/kopia/repo/splitter"

	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo"
)

const (
	defaultCacheLimitMB    = 5000
	maxCacheDurationSecond = 30
)

func setupLimits(ctx context.Context, flags map[string]string) throttling.Limits {
	return throttling.Limits{
		DownloadBytesPerSecond: optionalHaveFloat64(ctx, udmrepo.ThrottleOptionDownloadBytes, flags),
		ListsPerSecond:         optionalHaveFloat64(ctx, udmrepo.ThrottleOptionListOps, flags),
		ReadsPerSecond:         optionalHaveFloat64(ctx, udmrepo.ThrottleOptionReadOps, flags),
		UploadBytesPerSecond:   optionalHaveFloat64(ctx, udmrepo.ThrottleOptionUploadBytes, flags),
		WritesPerSecond:        optionalHaveFloat64(ctx, udmrepo.ThrottleOptionWriteOps, flags),
	}
}

// Helper function to choose between environment variable and default kopia algorithm value
func getKopiaAlgorithm(key, envKey string, flags map[string]string, supportedAlgorithms []string, defaultValue string) string {
	algorithm := os.Getenv(envKey)
	if len(algorithm) > 0 {
		if slices.Contains(supportedAlgorithms, algorithm) {
			return algorithm
		}
	}

	return optionalHaveStringWithDefault(key, flags, defaultValue)
}

// SetupNewRepositoryOptions setups the options when creating a new Kopia repository
func SetupNewRepositoryOptions(ctx context.Context, flags map[string]string) repo.NewRepositoryOptions {
	return repo.NewRepositoryOptions{
		BlockFormat: format.ContentFormat{
			Hash:       getKopiaAlgorithm(udmrepo.StoreOptionGenHashAlgo, "KOPIA_HASHING_ALGORITHM", flags, hashing.SupportedAlgorithms(), hashing.DefaultAlgorithm),
			Encryption: getKopiaAlgorithm(udmrepo.StoreOptionGenEncryptAlgo, "KOPIA_ENCRYPTION_ALGORITHM", flags, encryption.SupportedAlgorithms(false), encryption.DefaultAlgorithm),
		},

		ObjectFormat: format.ObjectFormat{
			Splitter: getKopiaAlgorithm(udmrepo.StoreOptionGenSplitAlgo, "KOPIA_SPLITTER_ALGORITHM", flags, splitter.SupportedAlgorithms(), splitter.DefaultAlgorithm),
		},

		RetentionMode:   blob.RetentionMode(optionalHaveString(udmrepo.StoreOptionGenRetentionMode, flags)),
		RetentionPeriod: optionalHaveDuration(ctx, udmrepo.StoreOptionGenRetentionPeriod, flags),
	}
}

// SetupConnectOptions setups the options when connecting to an existing Kopia repository
func SetupConnectOptions(ctx context.Context, repoOptions udmrepo.RepoOptions) repo.ConnectOptions {
	cacheLimit := optionalHaveIntWithDefault(ctx, udmrepo.StoreOptionCacheLimit, repoOptions.StorageOptions, defaultCacheLimitMB) << 20

	// 80% for data cache and 20% for metadata cache and align to KB
	dataCacheLimit := (cacheLimit / 5 * 4) >> 10
	metadataCacheLimit := (cacheLimit / 5) >> 10

	return repo.ConnectOptions{
		CachingOptions: content.CachingOptions{
			// softLimit 80%
			ContentCacheSizeBytes:  (dataCacheLimit / 5 * 4) << 10,
			MetadataCacheSizeBytes: (metadataCacheLimit / 5 * 4) << 10,
			// hardLimit 100%
			ContentCacheSizeLimitBytes:  dataCacheLimit << 10,
			MetadataCacheSizeLimitBytes: metadataCacheLimit << 10,
			MaxListCacheDuration:        content.DurationSeconds(time.Duration(maxCacheDurationSecond) * time.Second),
		},
		ClientOptions: repo.ClientOptions{
			Hostname:    optionalHaveString(udmrepo.GenOptionOwnerDomain, repoOptions.GeneralOptions),
			Username:    optionalHaveString(udmrepo.GenOptionOwnerName, repoOptions.GeneralOptions),
			ReadOnly:    optionalHaveBool(ctx, udmrepo.StoreOptionGenReadOnly, repoOptions.GeneralOptions),
			Description: repoOptions.Description,
		},
	}
}
