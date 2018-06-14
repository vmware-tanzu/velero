package plugin

import (
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

// HandlerInitializer is a function that initializes and returns a new instance of one of Ark's plugin interfaces
// (ObjectStore, BlockStore, BackupItemAction, RestoreItemAction).
type HandlerInitializer func(logger logrus.FieldLogger) (interface{}, error)

// serverMux manages multiple implementations of a single plugin kind, such as pod and pvc BackupItemActions.
type serverMux struct {
	kind         PluginKind
	initializers map[string]HandlerInitializer
	handlers     map[string]interface{}
	serverLog    logrus.FieldLogger
}

// newServerMux returns a new serverMux.
func newServerMux() *serverMux {
	return &serverMux{
		initializers: make(map[string]HandlerInitializer),
		handlers:     make(map[string]interface{}),
	}
}

// register registers the initializer for name.
func (m *serverMux) register(name string, f HandlerInitializer) {
	// TODO(ncdc): return an error on duplicate registrations for the same name.
	m.initializers[name] = f
}

// names returns a list of all registered implementations.
func (m *serverMux) names() []string {
	return sets.StringKeySet(m.initializers).List()
}

// setServerLog sets the server log to be used.
func (m *serverMux) setServerLog(log logrus.FieldLogger) {
	m.serverLog = log
}

// getHandler returns the instance for a plugin with the given name. If an instance has already been initialized,
// that is returned. Otherwise, the instance is initialized by calling its initialization function.
func (m *serverMux) getHandler(name string) (interface{}, error) {
	if instance, found := m.handlers[name]; found {
		return instance, nil
	}

	initializer, found := m.initializers[name]
	if !found {
		return nil, errors.Errorf("unknown %v plugin: %s", m.kind, name)
	}

	instance, err := initializer(m.serverLog)
	if err != nil {
		return nil, err
	}

	m.handlers[name] = instance

	return m.handlers[name], nil
}
