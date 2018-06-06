package logging

import (
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
	logger.Level = level

	for _, hook := range DefaultHooks() {
		logger.Hooks.Add(hook)
	}

	return logger
}
