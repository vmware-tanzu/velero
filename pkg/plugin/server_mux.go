package plugin

import (
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ServerInitializer is a function that initializes and returns a new instance of one of Ark's plugin interfaces
// (ObjectStore, BlockStore, BackupItemAction, RestoreItemAction).
type ServerInitializer func(logger logrus.FieldLogger) (interface{}, error)

// serverMux manages multiple implementations of a single plugin kind, such as pod and pvc BackupItemActions.
type serverMux struct {
	kind         PluginKind
	initializers map[string]ServerInitializer
	instances    map[string]interface{}
	serverLog    logrus.FieldLogger
}

// newServerMux returns a new serverMux.
func newServerMux() *serverMux {
	return &serverMux{
		initializers: make(map[string]ServerInitializer),
		instances:    make(map[string]interface{}),
	}
}

// register registers the initializer for name.
func (m *serverMux) register(name string, f ServerInitializer) {
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

// getInstance returns the instance for a plugin with the given name. If an instance has already been initialized,
// that is returned. Otherwise, the instance is initialized by calling its initialization function.
func (m *serverMux) getInstance(name string) (interface{}, error) {
	if instance, found := m.instances[name]; found {
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

	m.instances[name] = instance

	return m.instances[name], nil
}
