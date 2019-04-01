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
	"k8s.io/apimachinery/pkg/util/validation"
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

const hostnameRegexStringRFC952 = `^[a-zA-Z][a-zA-Z0-9\-\.]+[a-z-Az0-9]$` // https://tools.ietf.org/html/rfc952

func validPluginName(name string, existingNames []string) bool {
	errors := validation.IsQualifiedName(name)
	if len(errors) != 0 {
		return false
	}
	// if name == "" {
	// 	return false
	// }

	// tokens := strings.Split(name, "/")
	// fmt.Println("tokens are: ", tokens)

	// // hostnameRegexRFC952 := regexp.MustCompile(hostnameRegexStringRFC952)

	// fmt.Println("name before: ", name)
	// if name[len(name)-1] == '.' {
	// 	name = name[0 : len(name)-1]
	// }
	// fmt.Println("name after: ", name)

	// // return strings.ContainsAny(name, ".") &&
	// // 	hostnameRegexRFC952.MatchString(name)

	// name = strings.ToLower(name)

	// if len(tokens) <= 1 || tokens[0] == "" || tokens[1] == "" {
	// 	return false
	// }

	// if valid := prefixValidation(tokens[0]); !valid {
	// 	return false
	// }

	for _, existingName := range existingNames {
		if strings.Compare(name, existingName) == 0 {
			return false // found a duplicate
		}
	}
	return true
}

// valid characters of a-z, 0-9, ., -,
// max total length 253,
// max length between .'s 63

func prefixValidation(s string) bool {
	// s2 := strings.TrimSuffix(s, ".")
	// if s == s2 {
	// 	return false
	// }

	// i := strings.LastIndexFunc(s2, func(r rune) bool {
	// 	return r != '\\'
	// })

	// // Test whether we have an even number of escape sequences before
	// // the dot or none.
	// return (len(s2)-i)%2 != 0
	l := len(s)
	if l == 0 {
		return false
	}
	return s[l-1] == '.'
}
