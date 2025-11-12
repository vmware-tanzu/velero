/*
Copyright the Velero Contributors.

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

package backup

import "context"

// CheckCancelled checks if the context has been cancelled.
// Returns context.Canceled if cancelled, nil otherwise.
// This provides a consistent cancellation checkpoint across the backup code.
//
// This is used at safe checkpoints throughout the backup process to enable
// fast cancellation without blocking or deadlocks.
//
// If ctx is nil (e.g., in tests), this returns nil (no cancellation).
//
// Example usage:
//
//	if err := CheckCancelled(ctx); err != nil {
//	    log.Info("Backup cancelled")
//	    return partialResults
//	}
func CheckCancelled(ctx context.Context) error {
	// Handle nil context (e.g., in tests)
	if ctx == nil {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
