package plugin

import (
	"github.com/pkg/errors"
)

// registry is a simple store of plugin binary information. If a binary
// is registered as supporting multiple PluginKinds, it will be
// gettable/listable for all of those kinds.
type registry struct {
	// plugins is a nested map, keyed first by PluginKind,
	// and second by name. this is to allow easy listing
	// of plugins for a kind, as well as efficient lookup
	// of a plugin by kind+name.
	plugins map[PluginKind]map[string]pluginInfo
}

func newRegistry() *registry {
	return &registry{
		plugins: make(map[PluginKind]map[string]pluginInfo),
	}
}

// register adds a binary to the registry. If the binary supports multiple
// PluginKinds, it will be stored for each of those kinds so subsequent gets/lists
// for any supported kind will return it.
func (r *registry) register(name, commandName string, commandArgs []string, kinds ...PluginKind) {
	for _, kind := range kinds {
		if r.plugins[kind] == nil {
			r.plugins[kind] = make(map[string]pluginInfo)
		}

		r.plugins[kind][name] = pluginInfo{
			kinds:       kinds,
			name:        name,
			commandName: commandName,
			commandArgs: commandArgs,
		}
	}
}

// list returns info about all plugin binaries that implement the given
// PluginKind.
func (r *registry) list(kind PluginKind) ([]pluginInfo, error) {
	var res []pluginInfo

	if plugins, found := r.plugins[kind]; found {
		for _, itm := range plugins {
			res = append(res, itm)
		}

		return res, nil
	}

	return nil, errors.New("plugins not found")
}

// get returns info about a plugin with the given name and kind, or an
// error if one cannot be found.
func (r *registry) get(kind PluginKind, name string) (pluginInfo, error) {
	if forKind := r.plugins[kind]; forKind != nil {
		if plugin, found := r.plugins[kind][name]; found {
			return plugin, nil
		}
	}

	return pluginInfo{}, errors.New("plugin not found")
}
