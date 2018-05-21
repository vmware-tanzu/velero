package plugin

import (
	"sync"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// restartablePluginProcess encapsulates the lifecycle for all plugins contained in a single executable file. It is able
// to restart a plugin process if it is terminated for any reason. If this happens, all plugins are reinitialized using
// the original configuration data.
type restartablePluginProcess struct {
	builder *clientBuilder

	// lock guards all of the fields below
	lock           sync.RWMutex
	running        bool
	client         *plugin.Client
	protocolClient plugin.ClientProtocol
	plugins        map[kindAndName]interface{}
	reinitializers map[kindAndName]reinitializer
}

// reinitializer is capable of reinitializing a restartable plugin instance using the newly dispensed plugin.
type reinitializer interface {
	// reinitialize reinitializes a resumable plugin instance using the newly dispensed plugin.
	reinitialize(dispensed interface{}) error
}

// newRestartablePluginProcess creates a new restartablePluginProcess for the given command and options.
func newRestartablePluginProcess(command string, options ...restartablePluginProcessOption) (*restartablePluginProcess, error) {
	builder := newClientBuilder().withCommand(command)

	p := &restartablePluginProcess{
		builder:        builder,
		plugins:        make(map[kindAndName]interface{}),
		reinitializers: make(map[kindAndName]reinitializer),
	}

	for _, f := range options {
		f(p)
	}

	// This launches the process
	err := p.reset()

	return p, err
}

// restartablePluginProcessOption is a functional option for configuring p.
type restartablePluginProcessOption func(p *restartablePluginProcess)

// withLog is a restartablePluginProcessOption that configures the supplied logger and log level in the restartablePluginProcess's client builder.
func withLog(logger logrus.FieldLogger, level logrus.Level) restartablePluginProcessOption {
	return func(p *restartablePluginProcess) {
		p.builder.withLogger(&logrusAdapter{impl: logger, level: level})
	}
}

// addReinitializer registers the reinitializer r for key.
func (p *restartablePluginProcess) addReinitializer(key kindAndName, r reinitializer) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.reinitializers[key] = r
}

// reset acquires the lock and calls resetLH.
func (p *restartablePluginProcess) reset() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	return p.resetLH()
}

// resetLH (re)launches the plugin process. It redispenses all previously dispensed plugins and reinitializes all the
// registered reinitializers using the newly dispensed plugins.
//
// Callers of resetLH *must* acquire the lock before calling it.
func (p *restartablePluginProcess) resetLH() error {
	// This creates a new go-plugin Client that has its own unique exec.Cmd for launching the plugin process.
	p.client = p.builder.client()

	// This launches the plugin process.
	protocolClient, err := p.client.Client()
	if err != nil {
		return err
	}
	p.protocolClient = protocolClient

	// Redispense any previously dispensed plugins, reinitializing if necessary.
	// Start by creating a new map to hold the newly dispensed plugins.
	newPlugins := make(map[kindAndName]interface{})
	for key := range p.plugins {
		// Re-dispense
		dispensed, err := p.dispenseLH(key)
		if err != nil {
			return err
		}
		// Store in the new map
		newPlugins[key] = dispensed

		// Reinitialize
		if r, found := p.reinitializers[key]; found {
			if err := r.reinitialize(dispensed); err != nil {
				return err
			}
		}
	}

	// Make sure we update p's plugins!
	p.plugins = newPlugins

	return nil
}

// resetIfNeeded checks if the plugin process has exited and resets p if it has.
func (p *restartablePluginProcess) resetIfNeeded() (bool, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.client.Exited() {
		return true, p.resetLH()
	}

	return false, nil
}

// getByKindAndName acquires the lock and calls getByKindAndNameLH.
func (p *restartablePluginProcess) getByKindAndName(key kindAndName) (interface{}, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	return p.getByKindAndNameLH(key)
}

// getByKindAndNameLH returns the dispensed plugin for key. If the plugin hasn't been dispensed before, it dispenses a
// new one.
func (p *restartablePluginProcess) getByKindAndNameLH(key kindAndName) (interface{}, error) {
	dispensed, found := p.plugins[key]
	if found {
		return dispensed, nil
	}

	return p.dispenseLH(key)
}

// dispenseLH dispenses a plugin for key. If the dispensed plugin is a clientMux, dispenseLH retrieves the plugin
// instance for key.name.
func (p *restartablePluginProcess) dispenseLH(key kindAndName) (interface{}, error) {
	// This calls GRPCClient(clientConn) on the plugin instance registered for key.name.
	dispensed, err := p.protocolClient.Dispense(key.kind.String())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// Currently all plugins except for PluginLister dispense clientMux instances.
	if mux, ok := dispensed.(*clientMux); ok {
		// Get the instance that implements our plugin interface (e.g. cloudprovider.ObjectStore) that is a gRPC-based
		// client
		dispensed = mux.clientFor(key.name)
	}

	// Make sure we record what we dispensed!
	p.plugins[key] = dispensed

	return dispensed, nil
}

// stop terminates the plugin process.
func (p *restartablePluginProcess) stop() {
	p.lock.Lock()
	p.client.Kill()
	p.lock.Unlock()
}
