// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"github.com/pkg/errors"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

const UnknownDefaultErrStr = "unknown value to be set as default"

// SetDefaultsFunc is the built-in fn that saves the arguments to the local Starlark thread.
// Starlark format: set_defaults([ssh_config()][, kube_config()][, resources()])
func SetDefaultsFunc(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
	var val starlark.Value

	if args.Len() == 0 {
		return starlark.None, errors.New("atleast one of kube_config, ssh_config or resources is required")
	}

	iter := args.Iterate()
	defer iter.Done()
	for iter.Next(&val) {
		switch val.Type() {
		case "struct":
			constStr, err := GetConstructor(val)
			if err != nil {
				return starlark.None, errors.Wrap(err, UnknownDefaultErrStr)
			}
			if constStr == identifiers.kubeCfg {
				thread.SetLocal(identifiers.kubeCfg, val)
			} else if constStr == identifiers.sshCfg {
				thread.SetLocal(identifiers.sshCfg, val)
			} else {
				return starlark.None, errors.New(UnknownDefaultErrStr)
			}
		case "list":
			list := val.(*starlark.List)
			if list.Len() > 0 {
				resourceVal := list.Index(0)
				constStr, err := GetConstructor(resourceVal)
				if err != nil || constStr != identifiers.hostResource {
					return starlark.None, errors.Wrap(err, UnknownDefaultErrStr)
				}
				thread.SetLocal(identifiers.resources, list)
			}
		default:
			return starlark.None, errors.New(UnknownDefaultErrStr)
		}
	}

	return starlark.None, nil
}

func GetConstructor(val starlark.Value) (string, error) {
	s, ok := val.(*starlarkstruct.Struct)
	if !ok {
		return "", errors.New("cannot convert value to struct")
	}
	constructor, ok := s.Constructor().(starlark.String)
	if !ok {
		return "", errors.New("cannot convert constructor value to string")
	}
	return constructor.GoString(), nil
}
