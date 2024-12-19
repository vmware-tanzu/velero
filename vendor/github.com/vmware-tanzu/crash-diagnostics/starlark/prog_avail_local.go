// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"

	"github.com/vladimirvivien/gexe"
	"go.starlark.net/starlark"
)

// progAvailLocalFunc is a built-in starlark function that checks if a program is available locally.
// It returns the path to the command if availble or else, returns an empty string.
// Starlark format: prog_avail_local(prog=<prog_name>)
func progAvailLocalFunc(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var progStr string
	if err := starlark.UnpackArgs(
		identifiers.progAvailLocal, args, kwargs,
		"prog", &progStr,
	); err != nil {
		return starlark.None, fmt.Errorf("%s: %s", identifiers.progAvailLocal, err)
	}

	p := gexe.Prog().Avail(progStr)
	return starlark.String(p), nil
}
