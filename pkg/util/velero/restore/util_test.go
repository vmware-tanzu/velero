package restore

import (
	"testing"

	"github.com/stretchr/testify/require"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

func TestIsResourcePolicyValid(t *testing.T) {
	require.True(t, IsResourcePolicyValid(string(velerov1api.PolicyTypeNone)))
	require.True(t, IsResourcePolicyValid(string(velerov1api.PolicyTypeUpdate)))
	require.False(t, IsResourcePolicyValid(""))
}
