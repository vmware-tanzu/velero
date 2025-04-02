package config

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func TestGetDefaultConfig(t *testing.T) {
	config := GetDefaultConfig()
	assert.Equal(t, "0", config.PodResources.CPULimit)
}

func TestBindFlags(t *testing.T) {
	config := GetDefaultConfig()
	config.BindFlags(pflag.CommandLine)
	assert.Equal(t, "0", config.PodResources.CPULimit)
}
