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

package arkerrors

import (
	"github.com/pkg/errors"
)

type Code int32

const (
	Unknown Code = iota
	NotFound
)

type StackTracer interface {
	StackTrace() errors.StackTrace
}

// Error is an error that includes stack trace information.
// type Error interface {
// 	error
// 	StackTracer
// }

// Recover
func Recover(r interface{}) error {
	// var err Error

	// Check for a panic
	if r != nil {
		// Check if it's already an error
		if recoveredErr, ok := r.(error); !ok {
			// It's not an error, so use errors.Errorf to add a stack trace
			e := errors.Errorf("encountered a panic: %v", r)
			// err = e.(Error)
			return e
		} else {
			// Check if we need to wrap it with a stack trace
			if _, ok := recoveredErr.(StackTracer); !ok {
				e := errors.WithStack(recoveredErr)
				// err = e.(Error)
				return e
			} else {
				// err = recoveredErr.(Error)
				return recoveredErr
			}
		}
	}

	return nil
}

// func NewProtoError(code Code, err error) *proto.Error {
// 	if err == nil {
// 		return nil
// 	}

// 	return &proto.Error{
// 		Code:    int32(code),
// 		Message: err.Error(),
// 		Stack:   NewProtoStack(err),
// 	}
// }

// type protoErrorWithLocation struct {
// 	protoError    *proto.Error
// 	errorLocation ErrorLocation
// }

// func (e *protoErrorWithLocation) Error() string {
// 	return e.protoError.Message
// }

// func (e *protoErrorWithLocation) StackTrace() string {
// 	if e.protoError.Stack == nil {
// 		return ""
// 	}
// 	buf := new(strings.Builder)
// 	for _, frame := range e.protoError.Stack.Frames {
// 		fmt.Fprintf(buf, "%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)
// 	}
// 	return buf.String()
// }

// func (e *protoErrorWithLocation) ErrorLocation() ErrorLocation {
// 	return e.errorLocation
// }

// func NewErrorFromProto(protoErr *proto.Error) error {
// 	if protoErr == nil {
// 		return nil
// 	}

// 	err := &protoErrorWithLocation{
// 		protoError: protoErr,
// 	}

// 	if protoErr.Stack != nil {
// 		err.errorLocation = ErrorLocation{
// 			File:     protoErr.Stack.Frames[0].File,
// 			Line:     protoErr.Stack.Frames[0].Line,
// 			Function: protoErr.Stack.Frames[0].Function,
// 		}
// 	}

// 	return err
// }
