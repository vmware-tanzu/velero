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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const (
	AccessModeFileSystem = "by-file-system"
	AccessModeBlock      = "by-block-device"
	podGroupLabel        = "velero.io/exposer-pod-group"
	podGroupSnapshot     = "snapshot-exposer"
	SELinuxNone          = "none"
	SELinuxNoRelabeling  = "no-relabeling"
	SELinuxNoReadOnly    = "no-readonly"
)

// ExposeResult defines the result of expose.
// Varying from the type of the expose, the result may be different.
type ExposeResult struct {
	ByPod ExposeByPod
}

// ExposeByPod defines the result for the expose method that a hosting pod is created
type ExposeByPod struct {
	HostingPod       *corev1.Pod
	HostingContainer string
	VolumeName       string
}

// ValidateSelinuxDatamover validates if the input param is a valid SELinux approach for data mover.
// It will return an error if it's invalid.
func ValidateSELinuxDatamover(t string) error {
	t = strings.TrimSpace(t)
	if t != "" && t != SELinuxNone && t != SELinuxNoRelabeling && t != SELinuxNoReadOnly {
		return fmt.Errorf("invalid SELinux datamover option '%s', valid options are: '%s', '%s', '%s'", t, SELinuxNone, SELinuxNoRelabeling, SELinuxNoReadOnly)
	}

	return nil
}
