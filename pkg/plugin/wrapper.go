package plugin

import (
	"os"
	"sync"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// wrapper encapsulates the lifecycle for all plugins contained in a single executable file. It is able to restart a
// plugin process if it is terminated for any reason. If this happens, all plugins are reinitialized using the original
// configuration data.
type wrapper struct {
	builder *clientBuilder

	// lock guards all of the fields below
	lock           sync.RWMutex
	running        bool
	client         *plugin.Client
	protocolClient plugin.ClientProtocol
	plugins        map[kindAndName]interface{}
	reinitializers map[kindAndName]reinitializer
}

type reinitializer interface {
	reinitialize(dispensed interface{}) error
}

func newWrapper(command string, options ...wrapperOption) (*wrapper, error) {
	var args []string
	if command == os.Args[0] {
		args = append(args, "run-plugin")
	}
	builder := newClientBuilder().
		withCommand(command, args...)

	w := &wrapper{
		builder:        builder,
		plugins:        make(map[kindAndName]interface{}),
		reinitializers: make(map[kindAndName]reinitializer),
	}

	for _, f := range options {
		f(w)
	}

	err := w.reset()

	return w, err
}

type wrapperOption func(w *wrapper)

func withLog(logger logrus.FieldLogger, level logrus.Level) wrapperOption {
	return func(w *wrapper) {
		w.builder.withLogger(&logrusAdapter{impl: logger, level: level})
	}
}

func (w *wrapper) addReinitializer(key kindAndName, r reinitializer) {
	w.lock.Lock()
	defer w.lock.Unlock()

	w.reinitializers[key] = r
}

func (w *wrapper) reset() error {
	w.lock.Lock()
	defer w.lock.Unlock()

	return w.resetLH()
}

func (w *wrapper) resetLH() error {
	w.client = w.builder.client()

	protocolClient, err := w.client.Client()
	if err != nil {
		return err
	}
	w.protocolClient = protocolClient

	newPlugins := make(map[kindAndName]interface{})
	for key := range w.plugins {
		// re-dispense
		dispensed, err := w.dispenseLH(key)
		if err != nil {
			return err
		}
		newPlugins[key] = dispensed

		if r, found := w.reinitializers[key]; found {
			if err := r.reinitialize(dispensed); err != nil {
				return err
			}
		}
	}
	w.plugins = newPlugins

	return nil
}

func (w *wrapper) resetIfNeeded() (bool, error) {
	w.lock.Lock()
	defer w.lock.Unlock()

	if w.client.Exited() {
		return true, w.resetLH()
	}

	return false, nil
}

func (w *wrapper) getByKindAndName(key kindAndName) (interface{}, error) {
	w.lock.Lock()
	defer w.lock.Unlock()

	return w.getByKindAndNameLH(key)
}

func (w *wrapper) getByKindAndNameLH(key kindAndName) (interface{}, error) {
	dispensed, found := w.plugins[key]
	if found {
		return dispensed, nil
	}

	return w.dispenseLH(key)
}

func (w *wrapper) dispenseLH(key kindAndName) (interface{}, error) {
	dispensed, err := w.protocolClient.Dispense(key.kind.String())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if getByNamer, ok := dispensed.(GetByNamer); ok {
		dispensed = getByNamer.GetByName(key.name)
	}

	w.plugins[key] = dispensed

	return dispensed, nil
}

func (w *wrapper) stop() {
	w.lock.Lock()
	w.client.Kill()
	w.lock.Unlock()
}
