/*
Copyright 2017 the Heptio Ark contributors.

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

package collections

import (
	"strings"

	"github.com/pkg/errors"
)

// GetValue returns the object at root[path], where path is a dot separated string.
func GetValue(root map[string]interface{}, path string) (interface{}, error) {
	if root == nil {
		return "", errors.New("root is nil")
	}

	pathParts := strings.Split(path, ".")
	key := pathParts[0]

	obj, found := root[pathParts[0]]
	if !found {
		return "", errors.Errorf("key %v not found", pathParts[0])
	}

	if len(pathParts) == 1 {
		return obj, nil
	}

	subMap, ok := obj.(map[string]interface{})
	if !ok {
		return "", errors.Errorf("value at key %v is not a map[string]interface{}", key)
	}

	return GetValue(subMap, strings.Join(pathParts[1:], "."))
}

// GetString returns the string at root[path], where path is a dot separated string.
func GetString(root map[string]interface{}, path string) (string, error) {
	obj, err := GetValue(root, path)
	if err != nil {
		return "", err
	}

	str, ok := obj.(string)
	if !ok {
		return "", errors.Errorf("value at path %v is not a string", path)
	}

	return str, nil
}

// GetMap returns the map at root[path], where path is a dot separated string.
func GetMap(root map[string]interface{}, path string) (map[string]interface{}, error) {
	obj, err := GetValue(root, path)
	if err != nil {
		return nil, err
	}

	ret, ok := obj.(map[string]interface{})
	if !ok {
		return nil, errors.Errorf("value at path %v is not a map[string]interface{}", path)
	}

	return ret, nil
}

// GetSlice returns the slice at root[path], where path is a dot separated string.
func GetSlice(root map[string]interface{}, path string) ([]interface{}, error) {
	obj, err := GetValue(root, path)
	if err != nil {
		return nil, err
	}

	ret, ok := obj.([]interface{})
	if !ok {
		return nil, errors.Errorf("value at path %v is not a []interface{}", path)
	}

	return ret, nil
}

// ForEach calls fn on each object in the root[path] array, where path is a dot separated string.
func ForEach(root map[string]interface{}, path string, fn func(obj map[string]interface{}) error) error {
	s, err := GetSlice(root, path)
	if err != nil {
		return err
	}

	for i := range s {
		obj, ok := s[i].(map[string]interface{})
		if !ok {
			return errors.Errorf("unable to convert %s[%d] to an object", path, i)
		}
		if err := fn(obj); err != nil {
			return err
		}
	}

	return nil
}

// Exists returns true if root[path] exists, or false otherwise.
func Exists(root map[string]interface{}, path string) bool {
	if root == nil {
		return false
	}

	_, err := GetValue(root, path)
	return err == nil
}

// MergeMaps takes two map[string]string and merges missing keys from the second into the first.
// If a key already exists, its value is not overwritten.
func MergeMaps(first, second map[string]string) map[string]string {
	// If the first map passed in is empty, just use all of the second map's data
	if first == nil {
		first = map[string]string{}
	}

	for k, v := range second {
		_, ok := first[k]
		if !ok {
			first[k] = v
		}
	}

	return first
}
