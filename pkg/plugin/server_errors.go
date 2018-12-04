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
	"strconv"
	"strings"

	goproto "github.com/golang/protobuf/proto"
	"github.com/heptio/ark/pkg/arkerrors"
	proto "github.com/heptio/ark/pkg/plugin/generated"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// newGRPCError returns a grpc Status error with stack trace information (if possible). If err is already a grpc Status
// error, it is used and code is ignored.
func newGRPCError(code codes.Code, err error, details ...goproto.Message) error {
	// First, see if it's already a gRPC error
	st, ok := status.FromError(err)
	if !ok {
		// If not, create
		st = status.New(code, err.Error())
	}

	stack := protoStackFromError(err)
	if stack != nil {
		details = append(details, stack)
	}

	st, stErr := st.WithDetails(details...)
	if stErr != nil {
		return status.Error(codes.Unknown, fmt.Sprintf("error adding details to gRPC status: %v", stErr))
	}

	return st.Err()
}

// protoStackFromError constructs a *proto.Stack from the stack trace information contained in err, if available.
func protoStackFromError(err error) *proto.Stack {
	var stackTrace *proto.Stack

	if stackErr, ok := err.(arkerrors.StackTracer); ok {
		stackTrace = new(proto.Stack)

		trace := stackErr.StackTrace()
		for _, frame := range trace {
			stackTrace.Frames = append(stackTrace.Frames, protoStackFrameFromErrorsFrame(frame))
		}
	}

	return stackTrace
}

// protoStackFrameFromErrorsFrame converts an errors.Frame to a *proto.StackFrame.
func protoStackFrameFromErrorsFrame(frame errors.Frame) *proto.StackFrame {
	functionNameAndFileAndLine := fmt.Sprintf("%+v", frame)

	newLine := strings.Index(functionNameAndFileAndLine, "\n")
	functionName := functionNameAndFileAndLine[0:newLine]

	tab := strings.LastIndex(functionNameAndFileAndLine, "\t")
	fileAndLine := strings.Split(functionNameAndFileAndLine[tab+1:], ":")

	file := fileAndLine[0]

	line, err := strconv.Atoi(fileAndLine[1])
	if err != nil {
		// should never happen
		line = -1
	}

	return &proto.StackFrame{
		File:     file,
		Line:     int32(line),
		Function: functionName,
	}
}
