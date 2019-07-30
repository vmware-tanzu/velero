package backup

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequest_BackupResourceList(t *testing.T) {
	items := []itemKey{
		{
			resource:  "apps/v1/Deployment",
			name:      "my-deploy",
			namespace: "default",
		},
		{
			resource:  "v1/Pod",
			name:      "pod1",
			namespace: "ns1",
		},
		{
			resource:  "v1/Pod",
			name:      "pod2",
			namespace: "ns2",
		},
		{
			resource: "v1/PersistentVolume",
			name:     "my-pv",
		},
	}
	backedUpItems := map[itemKey]struct{}{}
	for _, it := range items {
		backedUpItems[it] = struct{}{}
	}

	req := Request{BackedUpItems: backedUpItems}
	assert.Equal(t, req.BackupResourceList(), map[string][]string{
		"apps/v1/Deployment":  []string{"default/my-deploy"},
		"v1/Pod":              []string{"ns1/pod1", "ns2/pod2"},
		"v1/PersistentVolume": []string{"my-pv"},
	})
}
