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

package plugin

import (
	"github.com/heptio/ark/pkg/cloudprovider"
	proto "github.com/heptio/ark/pkg/plugin/generated"
	"google.golang.org/grpc/status"
)

func notFoundErrorFromStatus(st *status.Status) error {
	for _, detail := range st.Details() {
		if nfe, ok := detail.(*proto.ObjectNotFound); ok {
			return cloudprovider.NewNotFoundError(nfe.Bucket, nfe.Key)
		}
	}

	return nil
}

