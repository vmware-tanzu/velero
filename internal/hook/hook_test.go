package hook

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddResult(t *testing.T) {
	results := &ResourceHookResults{
		RWMutex: &sync.RWMutex{},
		Results: []*ResourceHookResult{},
	}

	results.AddResult(&ResourceHookResult{Status: StatusCompleted})
	results.AddResult(&ResourceHookResult{Status: StatusFailed})
	results.AddResult(&ResourceHookResult{Status: StatusSkipped})
	assert.Equal(t, 3, results.Total)
	assert.Equal(t, 1, results.Completed)
	assert.Equal(t, 1, results.Failed)
	assert.Equal(t, 1, results.Skipped)
}
