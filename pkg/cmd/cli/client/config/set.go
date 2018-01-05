/*
Copyright 2018 the Heptio Ark contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func NewSetCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "set KEY=VALUE [KEY=VALUE]...",
		Short: "Set client configuration file values",
		Run: func(c *cobra.Command, args []string) {
			if len(args) < 1 {
				cmd.CheckError(errors.Errorf("At least one KEY=VALUE argument is required"))
			}

			config, err := client.LoadConfig()
			cmd.CheckError(err)

			for _, arg := range args {
				pair := strings.Split(arg, "=")
				if len(pair) != 2 {
					fmt.Fprintf(os.Stderr, "WARNING: invalid KEY=VALUE: %q\n", arg)
					continue
				}
				key, value := pair[0], pair[1]

				if value == "" {
					delete(config, key)
				} else {
					config[key] = value
				}
			}

			cmd.CheckError(client.SaveConfig(config))
		},
	}

	return c
}
