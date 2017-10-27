/*
Copyright 2017 Heptio Inc.

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
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/util/collections"
)

// backupPVAction inspects a PersistentVolumeClaim for the PersistentVolume
// that it references and backs it up
type backupPVAction struct {
}

func NewBackupPVAction() Action {
	return &backupPVAction{}
}

var pvGroupResource = schema.GroupResource{Group: "", Resource: "persistentvolumes"}

// Execute finds the PersistentVolume referenced by the provided
// PersistentVolumeClaim and backs it up
func (a *backupPVAction) Execute(log *logrus.Entry, item runtime.Unstructured, backup *v1.Backup) ([]ResourceIdentifier, error) {
	log.Info("Executing backupPVAction")
	var additionalItems []ResourceIdentifier

	pvc := item.UnstructuredContent()

	volumeName, err := collections.GetString(pvc, "spec.volumeName")
	if err != nil {
		return additionalItems, errors.WithMessage(err, "unable to get spec.volumeName")
	}

	additionalItems = append(additionalItems, ResourceIdentifier{
		GroupResource: pvGroupResource,
		Name:          volumeName,
	})

	return additionalItems, nil
}
