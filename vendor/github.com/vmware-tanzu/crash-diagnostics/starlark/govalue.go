// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// GoValue represents an inherent Go value which can be
// converted to a Starlark value/type
type GoValue struct {
	val interface{}
}

// NewGoValue creates a value with inherent Go type
func NewGoValue(val interface{}) *GoValue {
	return &GoValue{val: val}
}

// Value returns the original value as an interface{}
func (v *GoValue) Value() interface{} {
	return v.val
}

// ToStringDict converts map v to a starlark.StringDict value where the key is
// expected to be a string and the value to be a string, bool, numeric, or []T.
func (v *GoValue) ToStringDict() (starlark.StringDict, error) {
	result := make(starlark.StringDict)
	valType := reflect.TypeOf(v.val)
	valValue := reflect.ValueOf(v.val)

	switch valType.Kind() {
	case reflect.Map:
		if valType.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("ToStringDict failed assertion: %T requires string keys", v.val)
		}

		iter := valValue.MapRange()
		for iter.Next() {
			key := iter.Key()
			val := iter.Value()
			starVal, err := GoToStarlarkValue(val.Interface())
			if err != nil {
				return nil, fmt.Errorf("ToStringDict failed assertion: %s", err)
			}
			result[key.String()] = starVal
		}
	default:
		return nil, fmt.Errorf("ToStringDict does not support %T", v.val)
	}

	return result, nil
}

// ToDict converts map v to a *starlark.Dict value where the key and value can
// be of an arbitrary types of string, bool, numeric, or []T.
func (v *GoValue) ToDict() (*starlark.Dict, error) {
	valType := reflect.TypeOf(v.val)
	valValue := reflect.ValueOf(v.val)
	var dict *starlark.Dict

	switch valType.Kind() {
	case reflect.Map:
		dict = starlark.NewDict(valValue.Len())
		iter := valValue.MapRange()
		for iter.Next() {
			key, err := GoToStarlarkValue(iter.Key().Interface())
			if err != nil {
				return nil, fmt.Errorf("ToDict failed key conversion: %s", err)
			}

			val, err := GoToStarlarkValue(iter.Value().Interface())
			if err != nil {
				return nil, fmt.Errorf("ToDict failed value conversion: %s", err)
			}
			if err := dict.SetKey(key, val); err != nil {
				return nil, errors.Wrapf(err, "failed to add key: %s", key)
			}
		}
	default:
		return nil, fmt.Errorf("ToDict does not support %T", v.val)
	}

	return dict, nil
}

// ToList converts v of type []T to a *starlark.List value where the elements can
// be of an arbitrary types of string, bool, numeric, or []T.
func (v *GoValue) ToList() (*starlark.List, error) {
	valType := reflect.TypeOf(v.val)
	switch valType.Kind() {
	case reflect.Slice, reflect.Array:
		val, err := v.ToStarlarkValue()
		if err != nil {
			return nil, fmt.Errorf("ToList failed: %s", err)
		}
		elems, ok := val.(starlark.Tuple)
		if !ok {
			return nil, fmt.Errorf("ToList failed assertion: unexpected type: %T", val)
		}
		return starlark.NewList(elems), nil
	default:
		return nil, fmt.Errorf("ToList does not support %T", v.val)
	}

}

// ToTuple converts v of type []T to a starlark.Tuple value where the elements can
// be of an arbitrary types of string, bool, numeric, or []T.
func (v *GoValue) ToTuple() (starlark.Tuple, error) {
	valType := reflect.TypeOf(v.val)

	switch valType.Kind() {
	case reflect.Slice, reflect.Array:
		val, err := v.ToStarlarkValue()
		if err != nil {
			return nil, fmt.Errorf("ToList failed: %s", err)
		}
		return val.(starlark.Tuple), nil
	default:
		return nil, fmt.Errorf("ToList does not support %T", v.val)
	}

}

// ToStarlarkStruct converts a v of type struct or map to a *starlarkstruct.Struct value
func (v *GoValue) ToStarlarkStruct(constructorName string) (*starlarkstruct.Struct, error) {
	valType := reflect.TypeOf(v.val)
	valValue := reflect.ValueOf(v.val)
	constructor := starlark.String(constructorName)

	switch valType.Kind() {
	case reflect.Struct:
		stringDict := make(starlark.StringDict)
		for i := 0; i < valType.NumField(); i++ {
			fname := valType.Field(i).Name
			fval, err := GoToStarlarkValue(valValue.Field(i).Interface())
			if err != nil {
				return nil, fmt.Errorf("ToStarlarkStruct failed field value conversion: %s", err)
			}
			stringDict[fname] = fval
		}
		return starlarkstruct.FromStringDict(constructor, stringDict), nil
	case reflect.Map:
		stringDict, err := v.ToStringDict()
		if err != nil {
			return nil, fmt.Errorf("ToStarlarkStruct failed: %s", err)
		}
		return starlarkstruct.FromStringDict(constructor, stringDict), nil
	default:
		return nil, fmt.Errorf("ToDict does not support %T", v.val)
	}

}

func (v *GoValue) ToStarlarkValue() (starlark.Value, error) {
	return GoToStarlarkValue(v.val)
}

// GoToStarlarkValue converts Go value val to its Starlark value/type.
// It supports basic numeric types, string, bool, and slice/arrays.
func GoToStarlarkValue(val interface{}) (starlark.Value, error) {
	valType := reflect.TypeOf(val)
	valValue := reflect.ValueOf(val)
	switch valType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return starlark.MakeInt64(valValue.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return starlark.MakeUint64(valValue.Uint()), nil
	case reflect.Float32, reflect.Float64:
		return starlark.MakeInt64(valValue.Int()).Float(), nil
	case reflect.String:
		return starlark.String(valValue.String()), nil
	case reflect.Bool:
		return starlark.Bool(valValue.Bool()), nil
	case reflect.Slice, reflect.Array:
		var starElems []starlark.Value
		for i := 0; i < valValue.Len(); i++ {
			elemVal := valValue.Index(i)
			starElemVal, err := GoToStarlarkValue(elemVal.Interface())
			if err != nil {
				return starlark.None, err
			}
			starElems = append(starElems, starElemVal)
		}
		return starlark.Tuple(starElems), nil
	default:
		return starlark.None, fmt.Errorf("unable to assert Go type %T as Starlark type", val)
	}
}
