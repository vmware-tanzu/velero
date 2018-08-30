/*
Copyright 2018 the Heptio Ark contributors.

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

package plugin

import (
	"testing"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakePluginClient struct {
	terminated map[*fakePluginClient]bool
}

func (c *fakePluginClient) Kill() {
	c.terminated[c] = true
}

func (c *fakePluginClient) Client() (plugin.ClientProtocol, error) {
	panic("not implemented")
}

func TestCloseAllClients(t *testing.T) {
	var (
		store       = newClientStore()
		terminated  = make(map[*fakePluginClient]bool)
		clientCount = 0
	)

	for _, kind := range AllPluginKinds {
		for _, scope := range []string{"scope-1", "scope-2"} {
			for _, name := range []string{"name-1", "name-2"} {
				client := &fakePluginClient{terminated: terminated}
				terminated[client] = false
				store.add(client, kind, name, scope)
				clientCount++
			}
		}
	}

	// verify setup
	require.Len(t, terminated, clientCount)
	for _, status := range terminated {
		require.False(t, status)
	}

	m := &manager{clientStore: store}
	m.CloseAllClients()

	// we should have no additions to or removals from the `terminated` map
	assert.Len(t, terminated, clientCount)

	// all clients should have their entry in the `terminated` map flipped to true
	for _, status := range terminated {
		assert.True(t, status)
	}
	// the store's `clients` map should be empty
	assert.Len(t, store.clients, 0)
}

func TestCloseBackupItemActions(t *testing.T) {
	var (
		store                = newClientStore()
		terminated           = make(map[*fakePluginClient]bool)
		clientCount          = 0
		expectedTerminations = make(map[*fakePluginClient]bool)
		backupName           = "backup-1"
	)

	for _, kind := range AllPluginKinds {
		for _, scope := range []string{"backup-1", "backup-2"} {
			for _, name := range []string{"name-1", "name-2"} {
				client := &fakePluginClient{terminated: terminated}
				terminated[client] = false
				store.add(client, kind, name, scope)
				clientCount++

				if kind == PluginKindBackupItemAction && scope == backupName {
					expectedTerminations[client] = true
				}
			}
		}
	}

	// verify setup
	require.Len(t, terminated, clientCount)
	for _, status := range terminated {
		require.False(t, status)
	}

	m := &manager{clientStore: store}
	m.CloseBackupItemActions(backupName)

	// we should have no additions to or removals from the `terminated` map
	assert.Len(t, terminated, clientCount)

	// only those clients that we expected to be terminated should have
	// their entry in the `terminated` map flipped to true
	for client, status := range terminated {
		_, ok := expectedTerminations[client]
		assert.Equal(t, ok, status)
	}

	// clients for the kind/scope should have been removed
	_, err := store.list(PluginKindBackupItemAction, backupName)
	assert.EqualError(t, err, "clients not found")

	// total number of clients should decrease by the number of terminated
	// clients
	assert.Len(t, store.listAll(), clientCount-len(expectedTerminations))
}

func TestCloseRestoreItemActions(t *testing.T) {
	var (
		store                = newClientStore()
		terminated           = make(map[*fakePluginClient]bool)
		clientCount          = 0
		expectedTerminations = make(map[*fakePluginClient]bool)
		restoreName          = "restore-2"
	)

	for _, kind := range AllPluginKinds {
		for _, scope := range []string{"restore-1", "restore-2"} {
			for _, name := range []string{"name-1", "name-2"} {
				client := &fakePluginClient{terminated: terminated}
				terminated[client] = false
				store.add(client, kind, name, scope)
				clientCount++

				if kind == PluginKindRestoreItemAction && scope == restoreName {
					expectedTerminations[client] = true
				}
			}
		}
	}

	// verify setup
	require.Len(t, terminated, clientCount)
	for _, status := range terminated {
		require.False(t, status)
	}

	m := &manager{clientStore: store}
	m.CloseRestoreItemActions(restoreName)

	// we should have no additions to or removals from the `terminated` map
	assert.Len(t, terminated, clientCount)

	// only those clients that we expected to be terminated should have
	// their entry in the `terminated` map flipped to true
	for client, status := range terminated {
		_, ok := expectedTerminations[client]
		assert.Equal(t, ok, status)
	}

	// clients for the kind/scope should have been removed
	_, err := store.list(PluginKindRestoreItemAction, restoreName)
	assert.EqualError(t, err, "clients not found")

	// total number of clients should decrease by the number of terminated
	// clients
	assert.Len(t, store.listAll(), clientCount-len(expectedTerminations))
}
