/*
Copyright 2019 the Velero contributors.

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
package client

import (
	"os"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

// TestFactory tests the client.Factory interface.
func TestFactory(t *testing.T) {
	// Velero client configuration is currently omitted due to requiring a
	// test filesystem in pkg/test. This causes an import cycle as pkg/test
	// uses pkg/client's interfaces to implement fakes

	// Env variable should set the namespace if no config or argument are used
	os.Setenv("VELERO_NAMESPACE", "env-velero")
	f := NewFactory("velero", "", make(map[string]interface{}))

	assert.Equal(t, "env-velero", f.Namespace())

	os.Unsetenv("VELERO_NAMESPACE")

	// Argument should change the namespace
	f = NewFactory("velero", "", make(map[string]interface{}))
	s := "flag-velero"
	flags := new(pflag.FlagSet)

	f.BindFlags(flags)

	flags.Parse([]string{"--namespace", s})

	assert.Equal(t, s, f.Namespace())

	// An argument overrides the env variable if both are set.
	os.Setenv("VELERO_NAMESPACE", "env-velero")
	f = NewFactory("velero", "", make(map[string]interface{}))
	flags = new(pflag.FlagSet)

	f.BindFlags(flags)
	flags.Parse([]string{"--namespace", s})
	assert.Equal(t, s, f.Namespace())

	os.Unsetenv("VELERO_NAMESPACE")
}
