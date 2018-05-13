package plugin

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Registry manages information about available plugins.
type Registry interface {
	// DiscoverPlugins discovers all available plugins.
	DiscoverPlugins() error
	// SkipInternalDiscovery tells the registry not to discover plugins contained in the current executable, which is
	// typically the Ark server process. This is mainly used for testing and generally should not be needed elsewhere.
	SkipInternalDiscovery()
	// List returns all PluginIdentifiers for kind.
	List(kind PluginKind) []PluginIdentifier
	// Get returns the PluginIdentifier for kind and name.
	Get(kind PluginKind, name string) (PluginIdentifier, error)
}

// kindAndName is a convenience struct that combines a PluginKind and a name.
type kindAndName struct {
	kind PluginKind
	name string
}

// registry implements Registry.
type registry struct {
	// dir is the directory to search for plugins.
	dir string
	// skipInternalDiscovery prevents the registry from discovering plugins in the current executable. This is mainly
	// for testing purposes.
	skipInternalDiscovery bool
	logger                logrus.FieldLogger
	logLevel              logrus.Level
	pluginsByID           map[kindAndName]PluginIdentifier
	pluginsByKind         map[PluginKind][]PluginIdentifier
}

// NewRegistry returns a new registry.
func NewRegistry(dir string, logger logrus.FieldLogger, logLevel logrus.Level) Registry {
	return &registry{
		dir:           dir,
		logger:        logger,
		logLevel:      logLevel,
		pluginsByID:   make(map[kindAndName]PluginIdentifier),
		pluginsByKind: make(map[PluginKind][]PluginIdentifier),
	}
}

func (r *registry) SkipInternalDiscovery() {
	r.skipInternalDiscovery = true
}

func (r *registry) DiscoverPlugins() error {
	plugins, err := readPluginsDir(r.dir)
	if err != nil {
		return err
	}

	var commands []string
	if !r.skipInternalDiscovery {
		// ark's internal plugins
		commands = append(commands, os.Args[0])
	}
	commands = append(commands, plugins...)

	for _, command := range commands {
		plugins, err := r.listPlugins(command)
		if err != nil {
			// TODO(ncdc): should we log & skip over any errors trying to list plugins for an executable?
			return err
		}

		for _, plugin := range plugins {
			r.logger.WithFields(logrus.Fields{
				"kind":    plugin.Kind,
				"name":    plugin.Name,
				"command": command,
			}).Info("registering plugin")

			r.register(plugin)
		}
	}

	return nil
}

// List returns info about all plugin binaries that implement the given
// PluginKind.
func (r *registry) List(kind PluginKind) []PluginIdentifier {
	return r.pluginsByKind[kind]
}

// Get returns info about a plugin with the given name and kind, or an
// error if one cannot be found.
func (r *registry) Get(kind PluginKind, name string) (PluginIdentifier, error) {
	p, found := r.pluginsByID[kindAndName{kind: kind, name: name}]
	if !found {
		return PluginIdentifier{}, newPluginNotFoundError(kind, name)
	}
	return p, nil
}

// readPluginsDir recursively reads dir looking for plugins.
func readPluginsDir(dir string) ([]string, error) {
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, errors.WithStack(err)
	}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	fullPaths := make([]string, 0, len(files))
	for _, file := range files {
		fullPath := filepath.Join(dir, file.Name())

		if file.IsDir() {
			subDirPaths, err := readPluginsDir(fullPath)
			if err != nil {
				return nil, err
			}
			fullPaths = append(fullPaths, subDirPaths...)
			continue
		}

		if !executable(file) {
			continue
		}

		fullPaths = append(fullPaths, fullPath)
	}
	return fullPaths, nil
}

// executable determines if a file is executable.
func executable(info os.FileInfo) bool {
	/*
		When we AND the mode with 0111:

		- 0100 (user executable)
		- 0010 (group executable)
		- 0001 (other executable)

		the result will be 0 if and only if none of the executable bits is set.
	*/
	return (info.Mode() & 0111) != 0
}

// listPlugins executes command, queries it for registered plugins, and returns the list of PluginIdentifiers.
func (r *registry) listPlugins(command string) ([]PluginIdentifier, error) {
	process, err := newProcess(command, r.logger, r.logLevel)
	if err != nil {
		return nil, err
	}
	defer process.kill()

	plugin, err := process.dispense(kindAndName{kind: PluginKindPluginLister})
	if err != nil {
		return nil, err
	}

	lister, ok := plugin.(PluginLister)
	if !ok {
		return nil, errors.Errorf("%T is not a PluginLister", plugin)
	}

	return lister.ListPlugins()
}

// register registers a PluginIdentifier with the registry.
func (r *registry) register(id PluginIdentifier) {
	r.pluginsByID[kindAndName{kind: PluginKind(id.Kind), name: id.Name}] = id
	r.pluginsByKind[id.Kind] = append(r.pluginsByKind[id.Kind], id)
}

// pluginNotFoundError indicates a plugin could not be located for kind and name.
type pluginNotFoundError struct {
	kind PluginKind
	name string
}

// newPluginNotFoundError returns a new pluginNotFoundError for kind and name.
func newPluginNotFoundError(kind PluginKind, name string) *pluginNotFoundError {
	return &pluginNotFoundError{
		kind: kind,
		name: name,
	}
}

func (e *pluginNotFoundError) Error() string {
	return fmt.Sprintf("unable to locate %v plugin named %s", e.kind, e.name)
}
