/*
Copyright 2018 the Heptio Ark contributors.

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

package serverstatusrequest

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	velerov1api "github.com/heptio/velero/pkg/apis/velero/v1"
)

type Builder struct {
	serverStatusRequest velerov1api.ServerStatusRequest
}

func NewBuilder() *Builder {
	return &Builder{
		serverStatusRequest: velerov1api.ServerStatusRequest{
			TypeMeta: metav1.TypeMeta{
				APIVersion: velerov1api.SchemeGroupVersion.String(),
				Kind:       "ServerStatusRequest",
			},
		},
	}
}

func (b *Builder) Build() *velerov1api.ServerStatusRequest {
	return &b.serverStatusRequest
}

func (b *Builder) Namespace(namespace string) *Builder {
	b.serverStatusRequest.Namespace = namespace
	return b
}

func (b *Builder) Name(name string) *Builder {
	b.serverStatusRequest.Name = name
	return b
}

func (b *Builder) GenerateName(name string) *Builder {
	b.serverStatusRequest.GenerateName = name
	return b
}

func (b *Builder) Phase(phase velerov1api.ServerStatusRequestPhase) *Builder {
	b.serverStatusRequest.Status.Phase = phase
	return b
}

func (b *Builder) ProcessedTimestamp(time time.Time) *Builder {
	b.serverStatusRequest.Status.ProcessedTimestamp.Time = time
	return b
}

func (b *Builder) ServerVersion(version string) *Builder {
	b.serverStatusRequest.Status.ServerVersion = version
	return b
}
