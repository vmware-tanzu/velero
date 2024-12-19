// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vladimirvivien/gexe"
	"go.starlark.net/starlark"
)

// captureLocalFunc is a built-in starlark function that runs a provided command on the local machine.
// The output of the command is stored in a file at a specified location under the workdir directory.
// Starlark format: capture_local(cmd=<command> [,workdir=path][,file_name=name][,desc=description][,append=append])
func captureLocalFunc(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var cmdStr, workdir, fileName, desc string
	var append bool
	if err := starlark.UnpackArgs(
		identifiers.captureLocal, args, kwargs,
		"cmd", &cmdStr,
		"workdir?", &workdir,
		"file_name?", &fileName,
		"desc?", &desc,
		"append?", &append,
	); err != nil {
		return starlark.None, fmt.Errorf("%s: %s", identifiers.captureLocal, err)
	}

	if len(workdir) == 0 {
		dir, err := getWorkdirFromThread(thread)
		if err != nil {
			return starlark.None, err
		}
		workdir = dir
	}
	if len(fileName) == 0 {
		fileName = fmt.Sprintf("%s.txt", sanitizeStr(cmdStr))
	}

	filePath := filepath.Join(workdir, fileName)
	if err := os.MkdirAll(workdir, 0744); err != nil && !os.IsExist(err) {
		msg := fmt.Sprintf("%s error: %s", identifiers.captureLocal, err)
		return starlark.String(msg), nil
	}

	p := gexe.StartProc(cmdStr)
	// upon error, write error in file, return filepath
	if p.Err() != nil {
		msg := fmt.Sprintf("%s error: %s: %s", identifiers.captureLocal, p.Err(), p.Result())
		if err := captureOutput(strings.NewReader(msg), filePath, desc, append); err != nil {
			msg := fmt.Sprintf("%s error: %s", identifiers.captureLocal, err)
			return starlark.String(msg), nil
		}
		return starlark.String(filePath), nil
	}

	if err := captureOutput(p.Out(), filePath, desc, append); err != nil {
		msg := fmt.Sprintf("%s error: %s", identifiers.captureLocal, err)
		return starlark.String(msg), nil
	}

	return starlark.String(filePath), nil
}
