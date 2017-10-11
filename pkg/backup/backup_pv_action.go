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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/heptio/ark/pkg/util/collections"
)

// backupPVAction inspects a PersistentVolumeClaim for the PersistentVolume
// that it references and backs it up
type backupPVAction struct {
}

var _ Action = &backupPVAction{}

func NewBackupPVAction() Action {
	return &backupPVAction{}
}

// Execute finds the PersistentVolume referenced by the provided
// PersistentVolumeClaim and backs it up
func (a *backupPVAction) Execute(ctx *backupContext, pvc map[string]interface{}, backupper itemBackupper) error {
	pvcName, err := collections.GetString(pvc, "metadata.name")
	if err != nil {
		ctx.infof("unable to get metadata.name for PersistentVolumeClaim: %v", err)
		return err
	}

	volumeName, err := collections.GetString(pvc, "spec.volumeName")
	if err != nil {
		ctx.infof("unable to get spec.volumeName for PersistentVolumeClaim %s: %v", pvcName, err)
		return err
	}

	gvr, resource, err := ctx.discoveryHelper.ResourceFor(schema.GroupVersionResource{Resource: "persistentvolumes"})
	if err != nil {
		ctx.infof("error getting GroupVersionResource for PersistentVolumes: %v", err)
		return err
	}
	gr := gvr.GroupResource()

	client, err := ctx.dynamicFactory.ClientForGroupVersionResource(gvr, resource, "")
	if err != nil {
		ctx.infof("error getting client for GroupVersionResource=%s, Resource=%s: %v", gvr.String(), resource, err)
		return err
	}

	pv, err := client.Get(volumeName, metav1.GetOptions{})
	if err != nil {
		ctx.infof("error getting PersistentVolume %s: %v", volumeName, err)
		return errors.WithStack(err)
	}

	ctx.infof("backing up PersistentVolume %s for PersistentVolumeClaim %s", volumeName, pvcName)
	if err := backupper.backupItem(ctx, pv.UnstructuredContent(), gr); err != nil {
		ctx.infof("error backing up PersistentVolume %s: %v", volumeName, err)
		return err
	}

	return nil
}
