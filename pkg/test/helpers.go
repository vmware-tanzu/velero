/*
Copyright 2018 the Velero contributors.

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
	"encoding/json"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func UnstructuredOrDie(data string) *unstructured.Unstructured {
	o, _, err := unstructured.UnstructuredJSONScheme.Decode([]byte(data), nil, nil)
	if err != nil {
		panic(err)
	}
	return o.(*unstructured.Unstructured)
}

func GetAsMap(j string) (map[string]interface{}, error) {
	m := make(map[string]interface{})
	err := json.Unmarshal([]byte(j), &m)
	return m, err
}
