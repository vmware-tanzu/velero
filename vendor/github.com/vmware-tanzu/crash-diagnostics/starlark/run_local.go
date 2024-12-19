// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"

	"github.com/vladimirvivien/gexe"
	"go.starlark.net/starlark"
)

// runLocalFunc is a built-in starlark function that runs a provided command on the local machine.
// It returns the result of the command as struct containing information about the executed command.
// Starlark format: run_local(<command string>)
func runLocalFunc(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var cmdStr string
	if err := starlark.UnpackArgs(
		identifiers.runLocal, args, kwargs,
		"cmd", &cmdStr,
	); err != nil {
		return starlark.None, fmt.Errorf("%s: %s", identifiers.run, err)
	}

	p := gexe.RunProc(cmdStr)
	result := p.Result()
	if p.Err() != nil {
		result = fmt.Sprintf("%s error: %s: %s", identifiers.runLocal, p.Err(), p.Result())
	}

	return starlark.String(result), nil
}
