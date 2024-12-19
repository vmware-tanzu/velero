// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ReadArgsFile parses the args file and populates the map with the contents
// of that file. The parsing follows the following rules:
// * each line should contain only a single key=value pair
// * lines starting with # are ignored
// * empty lines are ignored
// * any line not following the above patterns are ignored with a warning message
func ReadArgsFile(path string, args map[string]string) error {
	path, err := ExpandPath(path)
	if err != nil {
		return err
	}

	file, err := os.Open(path)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("args file not found: %s", path))
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
		return err
	}

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "#") && len(strings.TrimSpace(line)) != 0 {
			if pair := strings.Split(line, "="); len(pair) == 2 {
				args[strings.TrimSpace(pair[0])] = strings.TrimSpace(pair[1])
			} else {
				logrus.Warnf("unknown entry in args file: %s", line)
			}
		}
	}

	return nil
}
