package flag

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringOfStringArray(t *testing.T) {
	array := NewStringArray("a", "b")
	assert.Equal(t, "a,b", array.String())
}

func TestSetOfStringArray(t *testing.T) {
	array := NewStringArray()
	require.NoError(t, array.Set("a,b"))
	assert.Equal(t, "a,b", array.String())
}

func TestTypeOfStringArray(t *testing.T) {
	array := NewStringArray()
	assert.Equal(t, "stringArray", array.Type())
}
