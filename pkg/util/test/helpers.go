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
