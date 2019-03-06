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
package framework

import (
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

// HandlerInitializer is a function that initializes and returns a new instance of one of Velero's plugin interfaces
// (ObjectStore, BlockStore, BackupItemAction, RestoreItemAction).
type HandlerInitializer func(logger logrus.FieldLogger) (interface{}, error)

// serverMux manages multiple implementations of a single plugin kind, such as pod and pvc ItemActions for backups.
type ServerMux struct {
	kind         PluginKind
	initializers map[string]HandlerInitializer
	Handlers     map[string]interface{}
	ServerLog    logrus.FieldLogger
}

// newServerMux returns a new serverMux.
func newServerMux(logger logrus.FieldLogger) *ServerMux {
	return &ServerMux{
		initializers: make(map[string]HandlerInitializer),
		Handlers:     make(map[string]interface{}),
		ServerLog:    logger,
	}
}

// register registers the initializer for name.
func (m *ServerMux) register(name string, f HandlerInitializer) {
	// TODO(ncdc): return an error on duplicate registrations for the same name.
	m.initializers[name] = f
}

// names returns a list of all registered implementations.
func (m *ServerMux) names() []string {
	return sets.StringKeySet(m.initializers).List()
}

// GetHandler returns the instance for a plugin with the given name. If an instance has already been initialized,
// that is returned. Otherwise, the instance is initialized by calling its initialization function.
func (m *ServerMux) GetHandler(name string) (interface{}, error) {
	if instance, found := m.Handlers[name]; found {
		return instance, nil
	}

	initializer, found := m.initializers[name]
	if !found {
		return nil, errors.Errorf("unknown %v plugin: %s", m.kind, name)
	}

	instance, err := initializer(m.ServerLog)
	if err != nil {
		return nil, err
	}

	m.Handlers[name] = instance

	return m.Handlers[name], nil
}
