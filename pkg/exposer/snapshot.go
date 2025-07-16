/*
Copyright The Velero Contributors.

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

package exposer

import (
	"context"
	"time"

	corev1api "k8s.io/api/core/v1"
)

// SnapshotExposer is the interfaces for a snapshot exposer
type SnapshotExposer interface {
	// Expose starts the process to expose a snapshot, the expose process may take long time
	Expose(context.Context, corev1api.ObjectReference, any) error
	// GetExposed polls the status of the expose.
	// If the expose is accessible by the current caller, it waits the expose ready and returns the expose result.
	// Otherwise, it returns nil as the expose result without an error.
	GetExposed(context.Context, corev1api.ObjectReference, time.Duration, any) (*ExposeResult, error)

	// PeekExposed tests the status of the expose.
	// If the expose is incomplete but not recoverable, it returns an error.
	// Otherwise, it returns nil immediately.
	PeekExposed(context.Context, corev1api.ObjectReference) error

	// DiagnoseExpose generate the diagnostic info when the expose is not finished for a long time.
	// If it finds any problem, it returns an string about the problem.
	DiagnoseExpose(context.Context, corev1api.ObjectReference) string

	// CleanUp cleans up any objects generated during the snapshot expose
	CleanUp(context.Context, corev1api.ObjectReference, string, string)
}
