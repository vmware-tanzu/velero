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

package udmrepo

const (
	StorageTypeS3    = "s3"
	StorageTypeAzure = "azure"
	StorageTypeFs    = "filesystem"
	StorageTypeGcs   = "gcs"

	GenOptionMaintainMode  = "mode"
	GenOptionMaintainFull  = "full"
	GenOptionMaintainQuick = "quick"

	StoreOptionS3KeyId            = "accessKeyID"
	StoreOptionS3Provider         = "providerName"
	StoreOptionS3SecretKey        = "secretAccessKey"
	StoreOptionS3Token            = "sessionToken"
	StoreOptionS3Endpoint         = "endpoint"
	StoreOptionS3DisableTls       = "doNotUseTLS"
	StoreOptionS3DisableTlsVerify = "skipTLSVerify"

	StoreOptionAzureKey            = "storageKey"
	StoreOptionAzureDomain         = "storageDomain"
	StoreOptionAzureStorageAccount = "storageAccount"
	StoreOptionAzureToken          = "sasToken"

	StoreOptionFsPath = "fspath"

	StoreOptionGcsReadonly = "readonly"

	StoreOptionOssBucket = "bucket"
	StoreOptionOssRegion = "region"

	StoreOptionCredentialFile = "credFile"
	StoreOptionPrefix         = "prefix"
	StoreOptionPrefixName     = "unified-repo"

	ThrottleOptionReadOps       = "readOPS"
	ThrottleOptionWriteOps      = "writeOPS"
	ThrottleOptionListOps       = "listOPS"
	ThrottleOptionUploadBytes   = "uploadBytes"
	ThrottleOptionDownloadBytes = "downloadBytes"
)
