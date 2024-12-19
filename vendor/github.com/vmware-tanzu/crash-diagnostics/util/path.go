// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"os"
	"path/filepath"
)

// ExpandPath converts the file path to include the home directory when prefixed with `~`.
func ExpandPath(path string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	if path[0] == '~' {
		path = filepath.Join(home, path[1:])
	}
	return path, nil
}
