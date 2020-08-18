/*
Copyright 2019 the Velero contributors.

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
// and hooks. The desired output format is passed as a LogFormat Enum.
func DefaultLogger(level logrus.Level, format Format) *logrus.Logger {
	logger := logrus.New()

	if format == FormatJSON {
		logger.Formatter = new(logrus.JSONFormatter)
		// Error hooks inject nested fields under "error.*" with the error
		// string message at "error".
		//
		// This structure is incompatible with recent Elasticsearch versions
		// where dots in field names are automatically expanded to objects;
		// field "error" cannot be both a string and an object at the same
		// time.
		//
		// ELK being a popular choice for log ingestion and a common reason
		// for enabling JSON logging in the first place, we avoid this mapping
		// problem by nesting the error's message at "error.message".
		//
		// This also follows the Elastic Common Schema (ECS) recommendation.
		// https://www.elastic.co/guide/en/ecs/current/ecs-error.html
		logrus.ErrorKey = "error.message"
	} else {
		logrus.ErrorKey = "error"
	}

	// Make sure the output is set to stdout so log messages don't show up as errors in cloud log dashboards.
	logger.Out = os.Stdout

	logger.Level = level

	for _, hook := range DefaultHooks() {
		logger.Hooks.Add(hook)
	}

	return logger
}
