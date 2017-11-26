package plugin

import (
	"sync"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/pkg/errors"
)

// clientKey is a unique ID for a plugin client.
type clientKey struct {
	kind PluginKind

	// scope is an additional identifier that allows multiple clients
	// for the same kind/name to be differentiated. It will typically
	// be the name of the applicable backup/restore for ItemAction
	// clients, and blank for Object/BlockStore clients.
	scope string
}

func newClientStore() *clientStore {
	return &clientStore{
		clients: make(map[clientKey]map[string]*plugin.Client),
		lock:    &sync.RWMutex{},
	}
}

// clientStore is a repository of active plugin clients.
type clientStore struct {
	// clients is a nested map, keyed first by clientKey (a
	// combo of kind and "scope"), and second by plugin name.
	// This enables easy listing of all clients for a given
	// kind and scope (e.g. all BackupItemActions for a given
	// backup), and efficient lookup by kind+name+scope (e.g.
	// the AWS ObjectStore.)
	clients map[clientKey]map[string]*plugin.Client
	lock    *sync.RWMutex
}

// get returns a plugin client for the given kind/name/scope, or an error if none
// is found.
func (s *clientStore) get(kind PluginKind, name, scope string) (*plugin.Client, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if forScope, found := s.clients[clientKey{kind, scope}]; found {
		if client, found := forScope[name]; found {
			return client, nil
		}
	}

	return nil, errors.New("client not found")
}

// list returns all plugin clients for the given kind/scope, or an
// error if none are found.
func (s *clientStore) list(kind PluginKind, scope string) ([]*plugin.Client, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if forScope, found := s.clients[clientKey{kind, scope}]; found {
		var clients []*plugin.Client

		for _, client := range forScope {
			clients = append(clients, client)
		}

		return clients, nil
	}

	return nil, errors.New("clients not found")
}

// add stores a plugin client for the given kind/name/scope.
func (s *clientStore) add(client *plugin.Client, kind PluginKind, name, scope string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	key := clientKey{kind, scope}

	if _, found := s.clients[key]; !found {
		s.clients[key] = make(map[string]*plugin.Client)
	}

	s.clients[key][name] = client
}

// delete removes the client with the given kind/name/scope from the store.
func (s *clientStore) delete(kind PluginKind, name, scope string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if forScope, found := s.clients[clientKey{kind, scope}]; found {
		delete(forScope, name)
	}
}

// deleteAll removes all clients with the given kind/scope from
// the store.
func (s *clientStore) deleteAll(kind PluginKind, scope string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.clients, clientKey{kind, scope})
}
