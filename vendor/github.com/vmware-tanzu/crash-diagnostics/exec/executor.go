// Copyright (c) 2019 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package exec

import (
	"fmt"
	"io"
	"os"

	"github.com/pkg/errors"
	"github.com/vmware-tanzu/crash-diagnostics/starlark"
)

type ArgMap map[string]string

func Execute(name string, source io.Reader, args ArgMap) error {
	star := starlark.New()

	if args != nil {
		starStruct, err := starlark.NewGoValue(args).ToStarlarkStruct("args")
		if err != nil {
			return err
		}

		star.AddPredeclared("args", starStruct)
	}

	err := star.Exec(name, source)
	if err != nil {
		err = errors.Wrap(err, "exec failed")
	}

	return err
}

func ExecuteFile(file *os.File, args ArgMap) error {
	return Execute(file.Name(), file, args)
}

type StarlarkModule struct {
	Name   string
	Source io.Reader
}

func ExecuteWithModules(name string, source io.Reader, args ArgMap, modules ...StarlarkModule) error {
	star := starlark.New()

	if args != nil {
		starStruct, err := starlark.NewGoValue(args).ToStarlarkStruct("args")
		if err != nil {
			return err
		}

		star.AddPredeclared("args", starStruct)
	}

	// load modules
	for _, mod := range modules {
		if err := star.Preload(mod.Name, mod.Source); err != nil {
			return fmt.Errorf("module load: %w", err)
		}
	}

	err := star.Exec(name, source)
	if err != nil {
		return fmt.Errorf("exec failed: %w", err)
	}

	return nil
}
