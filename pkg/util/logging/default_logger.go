/*
Copyright 2018 the Velero contributors.

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

package logging

import (
	"os"

	"github.com/sirupsen/logrus"
)

// DefaultHooks returns a slice of the default
// logrus hooks to be used by a logger.
func DefaultHooks() []logrus.Hook {
	return []logrus.Hook{
		&LogLocationHook{},
		&ErrorLocationHook{},
	}
}

// DefaultLogger returns a Logger with the default properties
// and hooks.
func DefaultLogger(level logrus.Level) *logrus.Logger {
	logger := logrus.New()

	// Make sure the output is set to stdout so log messages don't show up as errors in cloud log dashboards.
	logger.Out = os.Stdout

	logger.Level = level

	for _, hook := range DefaultHooks() {
		logger.Hooks.Add(hook)
	}

	return logger
}
