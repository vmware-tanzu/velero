// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"
	"log"

	"go.starlark.net/starlark"
)

// logFunc implements a starlark built-in func for simple message logging.
// This iteration uses Go's standard log package.
// Example:
//   log(msg="message", [prefix="info"])
func logFunc(t *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var msg string
	var prefix string
	if err := starlark.UnpackArgs(
		identifiers.log, args, kwargs,
		"msg", &msg,
		"prefix?", &prefix,
	); err != nil {
		return starlark.None, fmt.Errorf("%s: %s", identifiers.log, err)
	}

	// retrieve logger from thread
	loggerLocal := t.Local(identifiers.log)
	if loggerLocal == nil {
		addDefaultLogger(t)
		loggerLocal = t.Local(identifiers.log)
	}
	logger := loggerLocal.(*log.Logger)

	if prefix != "" {
		logger.Printf("%s: %s", prefix, msg)
	} else {
		logger.Print(msg)
	}

	return starlark.None, nil
}

func addDefaultLogger(t *starlark.Thread) {
	loggerLocal := t.Local(identifiers.log)
	if loggerLocal == nil {
		t.SetLocal(identifiers.log, log.Default())
	}
}
