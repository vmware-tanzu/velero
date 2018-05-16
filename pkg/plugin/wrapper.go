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

// reinitializer is capable of reinitializing a resumable plugin instance using the newly dispensed plugin.
type reinitializer interface {
	// reinitialize reinitializes a resumable plugin instance using the newly dispensed plugin.
	reinitialize(dispensed interface{}) error
}

// newWrapper creates a new wrapper for the given command and options.
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

	// This launches the process
	err := w.reset()

	return w, err
}

// wrapperOption is a functional option for configuring wrapper.
type wrapperOption func(w *wrapper)

// withLog is a wrapperOption that configures the supplied logger and log level in the wrapper's client builder.
func withLog(logger logrus.FieldLogger, level logrus.Level) wrapperOption {
	return func(w *wrapper) {
		w.builder.withLogger(&logrusAdapter{impl: logger, level: level})
	}
}

// addReinitializer registers the reinitializer r for key.
func (w *wrapper) addReinitializer(key kindAndName, r reinitializer) {
	w.lock.Lock()
	defer w.lock.Unlock()

	w.reinitializers[key] = r
}

// reset acquires the lock and calls resetLH.
func (w *wrapper) reset() error {
	w.lock.Lock()
	defer w.lock.Unlock()

	return w.resetLH()
}

// resetLH (re)launches the plugin process. It redispenses all previously dispensed plugins and reinitializes all the
// registered reinitializers using the newly dispensed plugins.
//
// Callers of resetLH *must* acquire the lock before calling it.
func (w *wrapper) resetLH() error {
	// This creates a new go-plugin Client that has its own unique exec.Cmd for launching the plugin process.
	w.client = w.builder.client()

	// This launches the plugin process.
	protocolClient, err := w.client.Client()
	if err != nil {
		return err
	}
	w.protocolClient = protocolClient

	// Redispense any previously dispensed plugins, reinitializing if necessary.
	newPlugins := make(map[kindAndName]interface{})
	for key := range w.plugins {
		// Re-dispense
		dispensed, err := w.dispenseLH(key)
		if err != nil {
			return err
		}
		newPlugins[key] = dispensed

		// Reinitialize
		if r, found := w.reinitializers[key]; found {
			if err := r.reinitialize(dispensed); err != nil {
				return err
			}
		}
	}
	// Make sure we update the wrapper's plugins!
	w.plugins = newPlugins

	return nil
}

// resetIfNeeded checks if the plugin process has exited and resets the wrapper if it has.
func (w *wrapper) resetIfNeeded() (bool, error) {
	w.lock.Lock()
	defer w.lock.Unlock()

	if w.client.Exited() {
		return true, w.resetLH()
	}

	return false, nil
}

// getByKindAndName acquires the lock and calls getByKindAndNameLH.
func (w *wrapper) getByKindAndName(key kindAndName) (interface{}, error) {
	w.lock.Lock()
	defer w.lock.Unlock()

	return w.getByKindAndNameLH(key)
}

// getByKindAndNameLH returns the dispensed plugin for key, dispensing a new plugin if it hasn't previously been
// dispensed.
func (w *wrapper) getByKindAndNameLH(key kindAndName) (interface{}, error) {
	dispensed, found := w.plugins[key]
	if found {
		return dispensed, nil
	}

	return w.dispenseLH(key)
}

// dispenseLH dispenses a plugin for key. If the dispensed plugin is a clientMux, dispenseLH retrieves the plugin
// instance for key.name.
func (w *wrapper) dispenseLH(key kindAndName) (interface{}, error) {
	dispensed, err := w.protocolClient.Dispense(key.kind.String())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if mux, ok := dispensed.(clientMux); ok {
		dispensed = mux.clientFor(key.name)
	}

	// Make sure we record what we dispensed!
	w.plugins[key] = dispensed

	return dispensed, nil
}

// clientMux allows a dispensed plugin to support multiple implementations, such as AWS and GCP object stores.
type clientMux interface {
	// clientFor returns a gRPC client for the plugin named name. Note, the return type is interface{} because there
	// isn't an interface for a gRPC client itself; the returned object must implement one of our plugin interfaces,
	// such as ObjectStore.
	clientFor(name string) interface{}
}

// stop terminates the plugin process.
func (w *wrapper) stop() {
	w.lock.Lock()
	w.client.Kill()
	w.lock.Unlock()
}
