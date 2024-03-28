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

import (
	"errors"
	"testing"
)

func TestGetSnapshotID(t *testing.T) {
	// Create a DataPathError instance for testing
	err := DataPathError{snapshotID: "123", err: errors.New("example error")}
	// Call the GetSnapshotID method to retrieve the snapshot ID
	snapshotID := err.GetSnapshotID()
	// Check if the retrieved snapshot ID matches the expected value
	if snapshotID != "123" {
		t.Errorf("GetSnapshotID() returned unexpected snapshot ID: got %s, want %s", snapshotID, "123")
	}
}

func TestError(t *testing.T) {
	// Create a DataPathError instance for testing
	err := DataPathError{snapshotID: "123", err: errors.New("example error")}
	// Call the Error method to retrieve the error message
	errMsg := err.Error()
	// Check if the retrieved error message matches the expected value
	expectedErrMsg := "example error"
	if errMsg != expectedErrMsg {
		t.Errorf("Error() returned unexpected error message: got %s, want %s", errMsg, expectedErrMsg)
	}
}
