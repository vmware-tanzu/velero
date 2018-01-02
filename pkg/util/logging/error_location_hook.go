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

package logging

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	errorFileField     = "error.file"
	errorFunctionField = "error.function"
)

// ErrorLocationHook is a logrus hook that attaches error location information
// to log entries if an error is being logged and it has stack-trace information
// (i.e. if it originates from or is wrapped by github.com/pkg/errors).
type ErrorLocationHook struct {
}

func (h *ErrorLocationHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *ErrorLocationHook) Fire(entry *logrus.Entry) error {
	var (
		errObj interface{}
		exists bool
	)

	if errObj, exists = entry.Data[logrus.ErrorKey]; !exists {
		return nil
	}

	err, ok := errObj.(error)
	if !ok {
		return errors.New("object logged as error does not satisfy error interface")
	}

	stackErr := getInnermostTrace(err)

	if stackErr != nil {
		stackTrace := stackErr.StackTrace()
		functionName := fmt.Sprintf("%n", stackTrace[0])
		fileAndLine := fmt.Sprintf("%s:%d", stackTrace[0], stackTrace[0])

		entry.Data[errorFileField] = fileAndLine
		entry.Data[errorFunctionField] = functionName
	}

	return nil
}

type stackTracer interface {
	error
	StackTrace() errors.StackTrace
}

type causer interface {
	Cause() error
}

// getInnermostTrace returns the innermost error that
// has a stack trace attached
func getInnermostTrace(err error) stackTracer {
	var tracer stackTracer

	for {
		t, isTracer := err.(stackTracer)
		if isTracer {
			tracer = t
		}

		c, isCauser := err.(causer)
		if isCauser {
			err = c.Cause()
		} else {
			return tracer
		}
	}
}
