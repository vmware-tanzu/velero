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

package ownership

import "github.com/vmware-tanzu/velero/pkg/repository/udmrepo"

const (
	defaultOwnerUsername = "default"
	defaultOwnerDomain   = "default"
)

// GetBackupOwner returns the owner used by uploaders when saving a snapshot or
// opening the unified repository. At present, use the default owner only
func GetBackupOwner() udmrepo.OwnershipOptions {
	return udmrepo.OwnershipOptions{
		Username:   defaultOwnerUsername,
		DomainName: defaultOwnerDomain,
	}
}

// GetBackupOwner returns the owner used to create/connect the unified repository.
//At present, use the default owner only
func GetRepositoryOwner() udmrepo.OwnershipOptions {
	return udmrepo.OwnershipOptions{
		Username:   defaultOwnerUsername,
		DomainName: defaultOwnerDomain,
	}
}
