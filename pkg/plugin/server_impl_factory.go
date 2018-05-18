package plugin

import (
	"github.com/heptio/ark/pkg/util/logging"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

type ServerImplFactory func() (interface{}, error)

type serverImplFactoryMux struct {
	initializers map[string]ServerImplFactory
	impls        map[string]interface{}
	serverLog    logrus.FieldLogger
}

func newServerImplFactoryMux() *serverImplFactoryMux {
	return &serverImplFactoryMux{
		initializers: make(map[string]ServerImplFactory),
		impls:        make(map[string]interface{}),
	}
}

func (m *serverImplFactoryMux) register(name string, f ServerImplFactory) {
	m.initializers[name] = f
}

func (m *serverImplFactoryMux) names() []string {
	return sets.StringKeySet(m.initializers).List()
}

func (m *serverImplFactoryMux) setServerLog(log logrus.FieldLogger) {
	m.serverLog = log
}

// getImpl returns the implementation for a plugin with the given name. If an instance has already been initialized,
// that is returned. Otherwise, the instance is initialized by calling its initialization function. If the instance
// implements logging.LogSetter, getImpl calls SetLog and passes the plugin process's logger to the instance.
func (m *serverImplFactoryMux) getImpl(name string) (interface{}, error) {
	if impl, found := m.impls[name]; found {
		return impl, nil
	}

	initializer, found := m.initializers[name]
	if !found {
		return nil, errors.Errorf("unknown plugin: %s", name)
	}

	impl, err := initializer()
	if err != nil {
		return nil, err
	}

	m.impls[name] = impl

	if logSetter, ok := impl.(logging.LogSetter); ok {
		logSetter.SetLog(m.serverLog)
	}

	return m.impls[name], nil
}
