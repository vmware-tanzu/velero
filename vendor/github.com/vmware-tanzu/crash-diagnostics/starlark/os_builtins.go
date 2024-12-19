// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"
	"os"
	"runtime"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

func setupOSStruct() *starlarkstruct.Struct {
	return starlarkstruct.FromStringDict(starlark.String(identifiers.os),
		starlark.StringDict{
			"name":     starlark.String(runtime.GOOS),
			"username": starlark.String(getUsername()),
			"home":     starlark.String(os.Getenv("HOME")),
			"getenv":   starlark.NewBuiltin("getenv", getEnvFunc),
		},
	)
}

func getEnvFunc(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if args == nil || args.Len() == 0 {
		return starlark.None, nil
	}
	key, ok := args.Index(0).(starlark.String)
	if !ok {
		return starlark.None, fmt.Errorf("os.getenv: invalid env key")
	}

	return starlark.String(os.Getenv(string(key))), nil
}
