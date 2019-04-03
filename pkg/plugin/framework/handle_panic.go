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

package framework

import (
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
)

// handlePanic is a panic handler for the server half of velero plugins.
func handlePanic(p interface{}) error {
	if p == nil {
		return nil
	}

	// If p is an error with a stack trace, we want to retain
	// it to preserve the stack trace. Otherwise, create a new
	// error here.
	var err error

	if panicErr, ok := p.(error); !ok {
		err = errors.Errorf("plugin panicked: %v", p)
	} else {
		if _, ok := panicErr.(stackTracer); ok {
			err = panicErr
		} else {
			err = errors.Wrap(panicErr, "plugin panicked")
		}
	}

	return newGRPCErrorWithCode(err, codes.Aborted)
}
