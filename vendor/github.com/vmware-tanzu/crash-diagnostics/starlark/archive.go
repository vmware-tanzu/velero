// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"

	"go.starlark.net/starlark"

	"github.com/vmware-tanzu/crash-diagnostics/archiver"
)

// archiveFunc is a built-in starlark function that bundles specified directories into
// an arhive format (i.e. tar.gz)
// Starlark format: archive(output_file=<file name> ,source_paths=list)
func archiveFunc(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var outputFile string
	var paths *starlark.List

	if err := starlark.UnpackArgs(
		identifiers.archive, args, kwargs,
		"output_file?", &outputFile,
		"source_paths", &paths,
	); err != nil {
		return starlark.None, fmt.Errorf("%s: %s", identifiers.archive, err)
	}

	if len(outputFile) == 0 {
		outputFile = "archive.tar.gz"
	}

	if paths != nil && paths.Len() == 0 {
		return starlark.None, fmt.Errorf("%s: one or more paths required", identifiers.archive)
	}

	if err := archiver.Tar(outputFile, getPathElements(paths)...); err != nil {
		return starlark.None, fmt.Errorf("%s failed: %s", identifiers.archive, err)
	}

	return starlark.String(outputFile), nil
}

func getPathElements(paths *starlark.List) []string {
	pathElems := []string{}
	for i := 0; i < paths.Len(); i++ {
		if val, ok := paths.Index(i).(starlark.String); ok {
			pathElems = append(pathElems, string(val))
		}
	}
	return pathElems
}
