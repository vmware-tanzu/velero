/*
Copyright 2017 the Heptio Ark contributors.

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

package plugin

import (
	"fmt"
	"log"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/sirupsen/logrus"
)

const pluginNameField = "pluginName"

// logrusAdapter implements the hclog.Logger interface and
// delegates all calls to a logrus logger.
type logrusAdapter struct {
	impl  logrus.FieldLogger
	level logrus.Level
	name  string
}

// args are alternating key, value pairs, where the keys
// are expected to be strings, and values can be any type.
func argsToFields(args ...interface{}) logrus.Fields {
	fields := make(map[string]interface{})

	for i := 0; i < len(args); i += 2 {
		switch args[i] {
		case "time", "timestamp", "level":
			// remove `time` & `timestamp` because this info will be added
			// by the Ark logger and we don't want to have duplicated
			// fields.
			//
			// remove `level` because it'll be added by the Ark logger based
			// on the call we make (and go-plugin is determining which level
			// to log at based on the hclog-compatible `@level` field which
			// we're adding via HcLogLevelHook).
		default:
			var val interface{}
			if i+1 < len(args) {
				val = args[i+1]
			}

			fields[fmt.Sprintf("%v", args[i])] = val
		}
	}

	return logrus.Fields(fields)
}

// Trace emits a message and key/value pairs at the DEBUG level
// (logrus doesn't have a TRACE level)
func (l *logrusAdapter) Trace(msg string, args ...interface{}) {
	l.Debug(msg, args...)
}

// Debug emits a message and key/value pairs at the DEBUG level
func (l *logrusAdapter) Debug(msg string, args ...interface{}) {
	l.impl.WithFields(argsToFields(args...)).Debug(msg)
}

// Info emits a message and key/value pairs at the INFO level
func (l *logrusAdapter) Info(msg string, args ...interface{}) {
	l.impl.WithFields(argsToFields(args...)).Info(msg)
}

// Warn emits a message and key/value pairs at the WARN level
func (l *logrusAdapter) Warn(msg string, args ...interface{}) {
	l.impl.WithFields(argsToFields(args...)).Warn(msg)
}

// Error emits a message and key/value pairs at the ERROR level
func (l *logrusAdapter) Error(msg string, args ...interface{}) {
	l.impl.WithFields(argsToFields(args...)).Error(msg)
}

// IsTrace indicates if TRACE logs would be emitted. This and the other Is* guards
// are used to elide expensive logging code based on the current level.
func (l *logrusAdapter) IsTrace() bool {
	return l.IsDebug()
}

// IsDebug indicates if DEBUG logs would be emitted. This and the other Is* guards
// are used to elide expensive logging code based on the current level.
func (l *logrusAdapter) IsDebug() bool {
	return l.level <= logrus.DebugLevel
}

// IsInfo indicates if INFO logs would be emitted. This and the other Is* guards
// are used to elide expensive logging code based on the current level.
func (l *logrusAdapter) IsInfo() bool {
	return l.level <= logrus.InfoLevel
}

// IsWarn indicates if WARN logs would be emitted. This and the other Is* guards
// are used to elide expensive logging code based on the current level.
func (l *logrusAdapter) IsWarn() bool {
	return l.level <= logrus.WarnLevel
}

// IsError indicates if ERROR logs would be emitted. This and the other Is* guards
// are used to elide expensive logging code based on the current level.
func (l *logrusAdapter) IsError() bool {
	return l.level <= logrus.ErrorLevel
}

// With creates a sublogger that will always have the given key/value pairs
func (l *logrusAdapter) With(args ...interface{}) hclog.Logger {
	return &logrusAdapter{
		impl:  l.impl.WithFields(argsToFields(args...)),
		level: l.level,
	}
}

// Named creates a logger that will add a `pluginName` field with the name string
// as the value. If the logger already has a name, the new value will be appended
// to the current name.
func (l *logrusAdapter) Named(name string) hclog.Logger {
	var newName string
	if l.name == "" {
		newName = name
	} else {
		newName = l.name + "." + name
	}

	return l.ResetNamed(newName)
}

// ResetNamed creates a logger that will add a `pluginName` field with the name string
// as the value. This sets the name of the logger to the value directly, unlike `Named`
// which appends the given value to the current name.
func (l *logrusAdapter) ResetNamed(name string) hclog.Logger {
	return &logrusAdapter{
		impl:  l.impl.WithField(pluginNameField, name),
		level: l.level,
		name:  name,
	}
}

// StandardLogger returns a value that conforms to the stdlib log.Logger interface
func (l *logrusAdapter) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	panic("not implemented")
}
