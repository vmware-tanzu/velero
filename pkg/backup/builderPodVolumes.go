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

package backup

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	velerov1api "github.com/heptio/velero/pkg/apis/velero/v1"
)

// PodVolumeBackupBuilder is a helper for concisely constructing PodVolumeBackup API objects.
type PodVolumeBackupBuilder struct {
	podVolumeBackup velerov1api.PodVolumeBackup
}

// NewPodVolumeBackupBuilder returns a PodVolumeBackupBuilder for a PodVolumeBackup with no namespace/name.
func NewPodVolumeBackupBuilder() *PodVolumeBackupBuilder {
	return NewNamedPodVolumeBackupBuilder("", "")
}

// NewNamedPodVolumeBackupBuilder returns a PodVolumeBackupBuilder for a Backup with the specified namespace
// and name.
func NewNamedPodVolumeBackupBuilder(namespace, name string) *PodVolumeBackupBuilder {
	return &PodVolumeBackupBuilder{
		podVolumeBackup: velerov1api.PodVolumeBackup{
			TypeMeta: metav1.TypeMeta{
				APIVersion: velerov1api.SchemeGroupVersion.String(),
				Kind:       "PodVolumeBackup",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
		},
	}
}

// PodVolumeBackup returns the built PodVolumeBackup API object.
func (p *PodVolumeBackupBuilder) PodVolumeBackup() *velerov1api.PodVolumeBackup {
	return &p.podVolumeBackup
}

// Namespace sets the PodVolumeBackup's namespace.
func (p *PodVolumeBackupBuilder) Namespace(namespace string) *PodVolumeBackupBuilder {
	p.podVolumeBackup.Namespace = namespace
	return p
}

// Name sets the PodVolumeBackup's name.
func (p *PodVolumeBackupBuilder) Name(name string) *PodVolumeBackupBuilder {
	p.podVolumeBackup.Name = name
	return p
}

// Labels sets the PodVolumeBackup's labels.
func (p *PodVolumeBackupBuilder) Labels(vals ...string) *PodVolumeBackupBuilder {
	if p.podVolumeBackup.Labels == nil {
		p.podVolumeBackup.Labels = map[string]string{}
	}

	// if we don't have an even number of values, e.g. a key and a value
	// for each pair, add an empty-string value at the end to serve as
	// the default value for the last key.
	if len(vals)%2 != 0 {
		vals = append(vals, "")
	}

	for i := 0; i < len(vals); i += 2 {
		p.podVolumeBackup.Labels[vals[i]] = vals[i+1]
	}
	return p
}

// Phase sets the PodVolumeBackup's phase.
func (p *PodVolumeBackupBuilder) Phase(phase velerov1api.PodVolumeBackupPhase) *PodVolumeBackupBuilder {
	p.podVolumeBackup.Status.Phase = phase
	return p
}
