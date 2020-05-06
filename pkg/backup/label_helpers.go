/*
Copyright 2020 the Velero contributors.

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

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/label"
)

// NewSelector returns a Selector based on the backup name.
// This is useful for interacting with Listers that need a Selector.
func NewSelector(name string) labels.Selector {
	return labels.SelectorFromSet(map[string]string{velerov1api.BackupNameLabel: label.GetValidName(name)})
}

// NewListOptions returns a ListOptions based on the backup name.
// This is useful for interacting with client-go clients that needs a ListOptions.
func NewListOptions(name string) metav1.ListOptions {
	return metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", velerov1api.BackupNameLabel, label.GetValidName(name)),
	}
}
