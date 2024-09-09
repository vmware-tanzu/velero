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

package itemblock

import (
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ItemBlock struct {
	Log   logrus.FieldLogger
	Items []ItemBlockItem
}

type ItemBlockItem struct {
	Gr           schema.GroupResource
	Item         *unstructured.Unstructured
	PreferredGVR schema.GroupVersionResource
}

func (ib *ItemBlock) AddUnstructured(gr schema.GroupResource, item *unstructured.Unstructured, preferredGVR schema.GroupVersionResource) {
	ib.Items = append(ib.Items, ItemBlockItem{
		Gr:           gr,
		Item:         item,
		PreferredGVR: preferredGVR,
	})
}

// Could return multiple items if EnableAPIGroupVersions is set. The item matching the preferredGVR is returned first
func (ib *ItemBlock) FindItem(gr schema.GroupResource, namespace, name string) []ItemBlockItem {
	var itemList []ItemBlockItem
	var returnList []ItemBlockItem

	for _, item := range ib.Items {
		if item.Gr == gr && item.Item != nil && item.Item.GetName() == name && item.Item.GetNamespace() == namespace {
			itemGV, err := schema.ParseGroupVersion(item.Item.GetAPIVersion())
			if err == nil && item.PreferredGVR.GroupVersion() == itemGV {
				returnList = append(returnList, item)
			} else {
				itemList = append(itemList, item)
			}
		}
	}
	return append(returnList, itemList...)
}
