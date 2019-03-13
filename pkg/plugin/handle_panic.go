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

package plugin

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// handlePanic is a panic handler for the server half of velero plugins.
func handlePanic(p interface{}) error {
	if p == nil {
		return nil
	}

	return status.Errorf(codes.Aborted, "plugin panicked: %v", p)
}
