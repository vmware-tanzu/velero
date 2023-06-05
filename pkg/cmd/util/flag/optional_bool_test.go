package flag

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringOfOptionalBool(t *testing.T) {
	// nil
	ob := NewOptionalBool(nil)
	assert.Equal(t, "<nil>", ob.String())

	// true
	b := true
	ob = NewOptionalBool(&b)
	assert.Equal(t, "true", ob.String())

	// false
	b = false
	ob = NewOptionalBool(&b)
	assert.Equal(t, "false", ob.String())
}

func TestSetOfOptionalBool(t *testing.T) {
	// error
	ob := NewOptionalBool(nil)
	assert.NotNil(t, ob.Set("invalid"))

	// nil
	ob = NewOptionalBool(nil)
	assert.Nil(t, ob.Set(""))
	assert.Nil(t, ob.Value)

	// true
	ob = NewOptionalBool(nil)
	assert.Nil(t, ob.Set("true"))
	assert.True(t, *ob.Value)
}

func TestTypeOfOptionalBool(t *testing.T) {
	ob := NewOptionalBool(nil)
	assert.Equal(t, "optionalBool", ob.Type())
}
