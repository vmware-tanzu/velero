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

package test

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/heptio/ark/pkg/apis/ark/v1"
)

type TestBackup struct {
	*v1.Backup
}

func NewTestBackup() *TestBackup {
	return &TestBackup{
		Backup: &v1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: v1.DefaultNamespace,
			},
		},
	}
}

func (b *TestBackup) WithNamespace(namespace string) *TestBackup {
	b.Namespace = namespace
	return b
}

func (b *TestBackup) WithName(name string) *TestBackup {
	b.Name = name
	return b
}

func (b *TestBackup) WithLabel(key, value string) *TestBackup {
	if b.Labels == nil {
		b.Labels = make(map[string]string)
	}
	b.Labels[key] = value

	return b
}

func (b *TestBackup) WithPhase(phase v1.BackupPhase) *TestBackup {
	b.Status.Phase = phase
	return b
}

func (b *TestBackup) WithIncludedResources(r ...string) *TestBackup {
	b.Spec.IncludedResources = r
	return b
}

func (b *TestBackup) WithExcludedResources(r ...string) *TestBackup {
	b.Spec.ExcludedResources = r
	return b
}

func (b *TestBackup) WithIncludedNamespaces(ns ...string) *TestBackup {
	b.Spec.IncludedNamespaces = ns
	return b
}

func (b *TestBackup) WithExcludedNamespaces(ns ...string) *TestBackup {
	b.Spec.ExcludedNamespaces = ns
	return b
}

func (b *TestBackup) WithTTL(ttl time.Duration) *TestBackup {
	b.Spec.TTL = metav1.Duration{Duration: ttl}
	return b
}

func (b *TestBackup) WithExpiration(expiration time.Time) *TestBackup {
	b.Status.Expiration = metav1.Time{Time: expiration}
	return b
}

func (b *TestBackup) WithVersion(version int) *TestBackup {
	b.Status.Version = version
	return b
}

func (b *TestBackup) WithSnapshot(pv string, snapshot string) *TestBackup {
	if b.Status.VolumeBackups == nil {
		b.Status.VolumeBackups = make(map[string]*v1.VolumeBackupInfo)
	}
	b.Status.VolumeBackups[pv] = &v1.VolumeBackupInfo{SnapshotID: snapshot}
	return b
}

func (b *TestBackup) WithSnapshotVolumes(value bool) *TestBackup {
	b.Spec.SnapshotVolumes = &value
	return b
}

func (b *TestBackup) WithSnapshotVolumesPointer(value *bool) *TestBackup {
	b.Spec.SnapshotVolumes = value
	return b
}
