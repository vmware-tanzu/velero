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
	"strings"

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
		// If there is no error field in the entry, return early.
		return nil
	}
	if _, exists := entry.Data[errorFileField]; exists {
		// If there is already an error file field, preserve it instead of overwriting it. This field will already exist if
		// the log message occurred in the server half of a plugin.
		return nil
	}
	if _, exists := entry.Data[errorFunctionField]; exists {
		// If there is already an error function field, preserve it instead of overwriting it. This field will already exist if
		// the log message occurred in the server half of a plugin.
		return nil
	}

	err, ok := errObj.(error)
	if !ok {
		return errors.Errorf("object logged as error does not satisfy error interface; type=%T", errObj)
	}

	// If the error is an errorLocationProvider, use that information. This applies to errors from plugins.
	if elp, ok := err.(errorLocationProvider); ok {
		el := elp.ErrorLocation()
		entry.Data[errorFileField] = removeArkPackagePrefix(el.FileAndLine())
		entry.Data[errorFunctionField] = el.Function
		return nil
	}

	stackErr := getInnermostTrace(err)

	if stackErr != nil {
		stackTrace := stackErr.StackTrace()
		// There is not a format specifier that gets the full path of the file
		// (see https://github.com/pkg/errors/issues/136). We use %+v which prints
		// out "FunctionName\n\tFile:Line" and then we parse it into each part.
		functionNameAndFileAndLine := fmt.Sprintf("%+v", stackTrace[0])
		newLine := strings.Index(functionNameAndFileAndLine, "\n")
		functionName := functionNameAndFileAndLine[0:newLine]
		tab := strings.LastIndex(functionNameAndFileAndLine, "\t")
		fileAndLine := removeArkPackagePrefix(functionNameAndFileAndLine[tab+1:])

		entry.Data[errorFileField] = fileAndLine
		entry.Data[errorFunctionField] = functionName
	}

	return nil
}

// errorLocationProvider returns an ErrorLocation.
type errorLocationProvider interface {
	ErrorLocation() ErrorLocation
}

// ErrorLocation represents the file, line, and function where an error occurred.
type ErrorLocation struct {
	File     string
	Line     int32
	Function string
}

// FileAndLine returns the string representation of the error location's file and line.
func (e ErrorLocation) FileAndLine() string {
	return fmt.Sprintf("%s:%d", e.File, e.Line)
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
