package restore

import (
	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

func IsResourcePolicyValid(resourcePolicy string) bool {
	if resourcePolicy == string(api.PolicyTypeNone) || resourcePolicy == string(api.PolicyTypeUpdate) {
		return true
	}
	return false
}
