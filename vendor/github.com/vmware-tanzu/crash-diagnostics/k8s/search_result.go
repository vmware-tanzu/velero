// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package k8s

import (
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SearchResult is the object representation of the kubernetes objects
// returned by querying the API server
type SearchResult struct {
	ListKind             string
	ResourceName         string
	ResourceKind         string
	GroupVersionResource schema.GroupVersionResource
	List                 *unstructured.UnstructuredList
	Namespaced           bool
	Namespace            string
}

// ToStarlarkValue converts the SearchResult object to a starlark dictionary
func (sr SearchResult) ToStarlarkValue() *starlarkstruct.Struct {
	var values []starlark.Value
	listDict := starlark.StringDict{}

	if sr.List != nil {
		for _, item := range sr.List.Items {
			values = append(values, convertToStruct(item))
		}
		listDict = starlark.StringDict{
			"Object": convertToStarlarkPrimitive(sr.List.Object),
			"Items":  starlark.NewList(values),
		}
	}
	listStruct := starlarkstruct.FromStringDict(starlarkstruct.Default, listDict)

	grValDict := starlark.StringDict{
		"Group":    starlark.String(sr.GroupVersionResource.Group),
		"Version":  starlark.String(sr.GroupVersionResource.Version),
		"Resource": starlark.String(sr.GroupVersionResource.Resource),
	}

	dict := starlark.StringDict{
		"ListKind":             starlark.String(sr.ListKind),
		"ResourceName":         starlark.String(sr.ResourceName),
		"ResourceKind":         starlark.String(sr.ResourceKind),
		"Namespaced":           starlark.Bool(sr.Namespaced),
		"Namespace":            starlark.String(sr.Namespace),
		"GroupVersionResource": starlarkstruct.FromStringDict(starlarkstruct.Default, grValDict),
		"List":                 listStruct,
	}

	return starlarkstruct.FromStringDict(starlark.String("search_result"), dict)
}

// convertToStruct returns a starlark struct constructed from the contents of the input.
func convertToStruct(obj unstructured.Unstructured) starlark.Value {
	return convertToStarlarkPrimitive(obj.Object)
}

// convertToStarlarkPrimitive returns a starlark value based on the Golang type passed
// as input to the function.
func convertToStarlarkPrimitive(input interface{}) starlark.Value {
	var value starlark.Value
	switch val := input.(type) {
	case string:
		value = starlark.String(val)
	case int, int32, int64:
		value = starlark.MakeInt64(val.(int64))
	case bool:
		value = starlark.Bool(val)
	case []interface{}:
		var structs []starlark.Value
		for _, i := range val {
			structs = append(structs, convertToStarlarkPrimitive(i))
		}
		value = starlark.NewList(structs)
	case map[string]interface{}:
		dict := starlark.StringDict{}
		for k, v := range val {
			dict[k] = convertToStarlarkPrimitive(v)
		}
		value = starlarkstruct.FromStringDict(starlarkstruct.Default, dict)
	default:
		value = starlark.None
	}
	return value
}
