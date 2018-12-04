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

package plugin

import (
	"fmt"
	"strings"

	proto "github.com/heptio/ark/pkg/plugin/generated"
	"github.com/heptio/ark/pkg/util/logging"
	"google.golang.org/grpc/status"
)

// fromServerError takes a gRPC error, extracts gRPC status details (if any) and passes them to all the detailHandlers
// (if any). If a detail handler returns a non-nil error, that becomes the error that fromServerError will return.
// Finally, if there is stack trace information encoded in the server error, attach that to the error to return and
// return it.
func fromServerError(err error, detailHandlers ...detailHandler) error {
	errorToReturn := err

	st := status.Convert(errorToReturn)

	details := st.Details()

	for _, h := range detailHandlers {
		updatedErr := h.handle(details)
		if updatedErr != nil {
			errorToReturn = updatedErr
		}
	}

	// If the status has stack details, make sure to include them
	if stack := stackFromDetails(details); stack != nil {
		errorToReturn = addStack(errorToReturn, stack)
	}

	return errorToReturn
}

// detailHandler handles gRPC status details.
type detailHandler interface {
	// handle handles gRPC status details. If any of the details is specific information about a strongly-typed error,
	// handle may return a new strongly-typed error with the expectation that this new error replaces any previous gRPC
	// error being examined.
	handle(details []interface{}) error
}

// detailHandlerFunc is an adapter that allows ordinary functions to act as detailHandlers.
type detailHandlerFunc func(details []interface{}) error

// handle invokes f with details.
func (f detailHandlerFunc) handle(details []interface{}) error {
	return f(details)
}

// errorWithProtoStack contains an error with optional stack and error location information.
type errorWithProtoStack struct {
	error
	stack         *proto.Stack
	errorLocation logging.ErrorLocation
}

// addStack augments err with stack and returns the updated error, or the original error if stack is nil.
func addStack(err error, stack *proto.Stack) error {
	if stack == nil {
		return err
	}

	return &errorWithProtoStack{
		error: err,
		stack: stack,
		errorLocation: logging.ErrorLocation{
			File:     stack.Frames[0].File,
			Line:     stack.Frames[0].Line,
			Function: stack.Frames[0].Function,
		},
	}
}

// grpcStatusProvider is capable of provided a grpc *status.Status.
type grpcStatusProvider interface {
	GRPCStatus() *status.Status
}

// isGRPCStatus returns true if err is a grpcStatusProvider.
func isGRPCStatus(err error) bool {
	_, ok := err.(grpcStatusProvider)
	return ok
}

// stackFromGRPCErr extracts a *proto.Stack from err, if err is a grpcStatusProvider. If err is a different type, or if
// the status's details do not contain a *proto.Stack, stackFromGRPCErr returns nil.
func stackFromGRPCErr(err error) *proto.Stack {
	if !isGRPCStatus(err) {
		return nil
	}

	st := err.(grpcStatusProvider).GRPCStatus()
	return stackFromGRPCStatus(st)
}

// stackFromGRPCStatus extracts a *proto.Stack from a grpc *status.Status. If the status's details do not contain a
// *proto.Stack, stackFromGRPCStatus returns nil.
func stackFromGRPCStatus(st *status.Status) *proto.Stack {
	return stackFromDetails(st.Details())
}

// stackFromDetails extracts a *proto.Stack from details. If the details do not contain a
// *proto.Stack, stackFromDetails returns nil.
func stackFromDetails(details []interface{}) *proto.Stack {
	for _, detail := range details {
		switch t := detail.(type) {
		case *proto.Stack:
			return t
		}
	}

	return nil
}

// StackTrace formats the stack as a pretty string.
func (e *errorWithProtoStack) StackTrace() string {
	if e.stack == nil {
		return ""
	}

	buf := new(strings.Builder)
	for _, frame := range e.stack.Frames {
		fmt.Fprintf(buf, "%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)
	}
	return buf.String()
}

// ErrorLocation returns the error location.
func (e *errorWithProtoStack) ErrorLocation() logging.ErrorLocation {
	return e.errorLocation
}
