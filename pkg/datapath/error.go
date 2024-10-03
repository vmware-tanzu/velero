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

package datapath

// DataPathError represents an error that occurred during a backup or restore operation
type DataPathError struct {
	snapshotID string
	err        error
}

// Error implements error.
func (e DataPathError) Error() string {
	return e.err.Error()
}

// GetSnapshotID returns the snapshot ID for the error.
func (e DataPathError) GetSnapshotID() string {
	return e.snapshotID
}
