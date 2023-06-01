package flag

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringOfEnum(t *testing.T) {
	enum := NewEnum("a", "a", "b", "c")
	assert.Equal(t, "a", enum.String())
}

func TestSetOfEnum(t *testing.T) {
	enum := NewEnum("a", "a", "b", "c")
	assert.NotNil(t, enum.Set("d"))

	require.Nil(t, enum.Set("b"))
	assert.Equal(t, "b", enum.String())
}

func TestTypeOfEnum(t *testing.T) {
	enum := NewEnum("a", "a", "b", "c")
	assert.Equal(t, "", enum.Type())
}

func TestAllowedValuesOfEnum(t *testing.T) {
	enum := NewEnum("a", "a", "b", "c")
	assert.Equal(t, []string{"a", "b", "c"}, enum.AllowedValues())
}
