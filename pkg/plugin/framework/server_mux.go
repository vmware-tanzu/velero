/*
Copyright 2018, 2019 the Velero contributors.

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
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

// HandlerInitializer is a function that initializes and returns a new instance of one of Velero's plugin interfaces
// (ObjectStore, VolumeSnapshotter, BackupItemAction, RestoreItemAction).
type HandlerInitializer func(logger logrus.FieldLogger) (interface{}, error)

// serverMux manages multiple implementations of a single plugin kind, such as pod and pvc BackupItemActions.
type serverMux struct {
	kind         PluginKind
	initializers map[string]HandlerInitializer
	handlers     map[string]interface{}
	serverLog    logrus.FieldLogger
}

// newServerMux returns a new serverMux.
func newServerMux(logger logrus.FieldLogger) *serverMux {
	return &serverMux{
		initializers: make(map[string]HandlerInitializer),
		handlers:     make(map[string]interface{}),
		serverLog:    logger,
	}
}

// register registers the initializer for the given name. Proper format
// for the name is <namespace>/<name> and there can be no duplicates.
func (m *serverMux) register(name string, f HandlerInitializer) {
	if !validPluginName(name, m.names()) {
		return
	}
	m.initializers[name] = f
}

// names returns a list of all registered implementations.
func (m *serverMux) names() []string {
	return sets.StringKeySet(m.initializers).List()
}

// getHandler returns the instance for a plugin with the given name. If an instance has already been initialized,
// that is returned. Otherwise, the instance is initialized by calling its initialization function.
func (m *serverMux) getHandler(name string) (interface{}, error) {
	if instance, found := m.handlers[name]; found {
		return instance, nil
	}

	initializer, found := m.initializers[name]
	if !found {
		return nil, errors.Errorf("%v plugin: %s was not found or is an invalid format. Valid format: <namespace>/<name>", m.kind, name)
	}

	instance, err := initializer(m.serverLog)
	if err != nil {
		return nil, err
	}

	m.handlers[name] = instance

	return m.handlers[name], nil
}

func validPluginName(name string, existingNames []string) bool {
	name = strings.ToLower(name)
	tokens := strings.Split(name, "/")
	if len(tokens) <= 1 || tokens[0] == "" || tokens[1] == "" {
		return false
	}

	for _, existingName := range existingNames {
		if strings.Compare(name, existingName) == 0 {
			return false // found a duplicate
		}
	}
	return true
}
