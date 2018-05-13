package plugin

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Registry interface {
	DiscoverPlugins() error
	List(kind PluginKind) []PluginIdentifier
	Get(kind PluginKind, name string) (PluginIdentifier, error)
}

type kindAndName struct {
	kind PluginKind
	name string
}

// registry is a simple store of plugin binary information. If a binary
// is registered as supporting multiple PluginKinds, it will be
// gettable/listable for all of those kinds.
type registry struct {
	dir    string
	logger logrus.FieldLogger
	// logLevel      logrus.Level
	pluginsByID   map[kindAndName]PluginIdentifier
	pluginsByKind map[PluginKind][]PluginIdentifier
}

func NewRegistry(dir string, logger logrus.FieldLogger /*, logLevel logrus.Level*/) Registry {
	return &registry{
		dir:    dir,
		logger: logger,
		// logLevel:      logLevel,
		pluginsByID:   make(map[kindAndName]PluginIdentifier),
		pluginsByKind: make(map[PluginKind][]PluginIdentifier),
	}
}

func (r *registry) readPluginsDir() ([]string, error) {
	if _, err := os.Stat(r.dir); err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, errors.WithStack(err)
	}

	files, err := ioutil.ReadDir(r.dir)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	fullPaths := make([]string, 0, len(files))
	for _, file := range files {
		fullPaths = append(fullPaths, filepath.Join(r.dir, file.Name()))
	}
	return fullPaths, nil
}

func (r *registry) DiscoverPlugins() error {
	plugins, err := r.readPluginsDir()
	if err != nil {
		return err
	}

	commands := []string{os.Args[0]} // ark itself
	commands = append(commands, plugins...)

	for _, command := range commands {
		plugins, err := r.listPlugins(command)
		if err != nil {
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

func (r *registry) listPlugins(command string) ([]PluginIdentifier, error) {
	// logger := &logrusAdapter{impl: r.logger, level: r.logLevel}

	var args []string
	if command == os.Args[0] {
		args = append(args, "run-plugin")
	}

	builder := newClientBuilder().
		withCommand(command, args...)
		// withLogger(logger)

	client := builder.client()

	protocolClient, err := client.Client()
	if err != nil {
		return nil, err
	}

	plugin, err := protocolClient.Dispense(string(PluginKindPluginLister))
	if err != nil {
		return nil, err
	}

	lister, ok := plugin.(PluginLister)
	if !ok {
		return nil, errors.Errorf("%T is not a PluginLister")
	}

	return lister.ListPlugins()
}

// register adds a binary to the registry. If the binary supports multiple
// PluginKinds, it will be stored for each of those kinds so subsequent gets/lists
// for any supported kind will return it.
func (r *registry) register(id PluginIdentifier) {
	r.pluginsByID[kindAndName{kind: PluginKind(id.Kind), name: id.Name}] = id
	r.pluginsByKind[id.Kind] = append(r.pluginsByKind[id.Kind], id)
}

// list returns info about all plugin binaries that implement the given
// PluginKind.
func (r *registry) List(kind PluginKind) []PluginIdentifier {
	return r.pluginsByKind[kind]
}

// get returns info about a plugin with the given name and kind, or an
// error if one cannot be found.
func (r *registry) Get(kind PluginKind, name string) (PluginIdentifier, error) {
	p, found := r.pluginsByID[kindAndName{kind: kind, name: name}]
	if !found {
		return PluginIdentifier{}, newPluginNotFoundError(kind, name)
	}
	return p, nil
}

type pluginNotFoundError struct {
	kind PluginKind
	name string
}

func newPluginNotFoundError(kind PluginKind, name string) *pluginNotFoundError {
	return &pluginNotFoundError{
		kind: kind,
		name: name,
	}
}

func (e *pluginNotFoundError) Error() string {
	return fmt.Sprintf("unable to locate %v plugin named %s", e.kind, e.name)
}
