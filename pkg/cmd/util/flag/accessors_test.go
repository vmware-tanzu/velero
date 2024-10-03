package flag

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestGetOptionalStringFlag(t *testing.T) {
	flagName := "flag"

	// not specified
	cmd := &cobra.Command{}
	assert.Equal(t, "", GetOptionalStringFlag(cmd, flagName))

	// specified
	cmd.Flags().String(flagName, "value", "")
	assert.Equal(t, "value", GetOptionalStringFlag(cmd, flagName))
}

func TestGetOptionalBoolFlag(t *testing.T) {
	flagName := "flag"

	// not specified
	cmd := &cobra.Command{}
	assert.False(t, GetOptionalBoolFlag(cmd, flagName))

	// specified
	cmd.Flags().Bool(flagName, true, "")
	assert.True(t, GetOptionalBoolFlag(cmd, flagName))
}

func TestGetOptionalStringArrayFlag(t *testing.T) {
	flagName := "flag"

	// not specified
	cmd := &cobra.Command{}
	assert.Equal(t, []string{}, GetOptionalStringArrayFlag(cmd, flagName))

	// specified
	values := NewStringArray("value")
	cmd.Flags().Var(&values, flagName, "")
	assert.Equal(t, []string{"value"}, GetOptionalStringArrayFlag(cmd, flagName))
}
