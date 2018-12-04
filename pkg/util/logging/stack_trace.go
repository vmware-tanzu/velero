/*
Copyright 2018 the Heptio Ark contributors.

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
	"github.com/heptio/ark/pkg/arkerrors"
	"github.com/sirupsen/logrus"
)

type stackTraceStringer interface {
	StackTrace() string
}

// LogStackTrace attempts to log err's stack trace details at the debug log level. If err does not contain either
// gRPC-based or pkg/errors-based stack trace information, or the log level is higher than debug, LogStackTrace does
// nothing.
func LogStackTrace(log logrus.FieldLogger, err error) {
	switch t := err.(type) {
	case stackTraceStringer:
		log.WithError(err).Debug(t.StackTrace())
	case arkerrors.StackTracer:
		log.WithError(err).Debugf("%+v", t.StackTrace())
	}
}
