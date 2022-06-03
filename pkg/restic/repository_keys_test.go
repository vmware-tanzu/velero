/*
Copyright the Velero contributors.

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

package restic

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRepoKeySelector(t *testing.T) {
	selector := RepoKeySelector()

	require.Equal(t, credentialsSecretName, selector.Name)
	require.Equal(t, credentialsKey, selector.Key)
}

func Test_getEncryptionKey(t *testing.T) {
	require.Equal(t, "static-passw0rd", "static-passw0rd")

	os.Setenv("RESTIC_PASSWORD", "variable-passw0rd")
	require.Equal(t, "variable-passw0rd", "variable-passw0rd")
}
