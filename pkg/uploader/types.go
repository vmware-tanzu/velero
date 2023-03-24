/*
Copyright the Velero contributors.

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

package uploader

import (
	"fmt"
	"strings"
)

const (
	ResticType = "restic"
	KopiaType  = "kopia"
)

// ValidateUploaderType validates if the input param is a valid uploader type.
// It will return an error if it's invalid.
func ValidateUploaderType(t string) error {
	t = strings.TrimSpace(t)
	if t != ResticType && t != KopiaType {
		return fmt.Errorf("invalid uploader type '%s', valid upload types are: '%s', '%s'", t, ResticType, KopiaType)
	}
	return nil
}

type SnapshotInfo struct {
	ID   string `json:"id"`
	Size int64  `json:"Size"`
}

// UploaderProgress which defined two variables to record progress
type UploaderProgress struct {
	TotalBytes int64 `json:"totalBytes,omitempty"`
	BytesDone  int64 `json:"doneBytes,omitempty"`
}

// UploaderProgress which defined generic interface to update progress
type ProgressUpdater interface {
	UpdateProgress(p *UploaderProgress)
}
