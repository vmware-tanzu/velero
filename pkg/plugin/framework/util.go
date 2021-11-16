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

package framework

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	proto "github.com/vmware-tanzu/velero/pkg/plugin/generated"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

func packResourceIdentifiers(resourcesIDs []velero.ResourceIdentifier) (protoIDs []*proto.ResourceIdentifier) {
	for _, item := range resourcesIDs {
		protoIDs = append(protoIDs, resourceIdentifierToProto(item))
	}
	return
}

func unpackResourceIdentifiers(protoIDs []*proto.ResourceIdentifier) (resourceIDs []velero.ResourceIdentifier) {
	for _, itm := range protoIDs {
		resourceIDs = append(resourceIDs, protoToResourceIdentifier(itm))
	}
	return
}

func protoToResourceIdentifier(proto *proto.ResourceIdentifier) velero.ResourceIdentifier {
	return velero.ResourceIdentifier{
		GroupResource: schema.GroupResource{
			Group:    proto.Group,
			Resource: proto.Resource,
		},
		Namespace: proto.Namespace,
		Name:      proto.Name,
	}
}

func resourceIdentifierToProto(id velero.ResourceIdentifier) *proto.ResourceIdentifier {
	return &proto.ResourceIdentifier{
		Group:     id.Group,
		Resource:  id.Resource,
		Namespace: id.Namespace,
		Name:      id.Name,
	}
}
