package logging

import "github.com/sirupsen/logrus"

// LogSetter is an interface for a type that allows a FieldLogger
// to be set on it.
type LogSetter interface {
	SetLog(logrus.FieldLogger)
}
