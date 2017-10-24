/*
Copyright 2017 Heptio Inc.

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

package version

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/heptio/ark/pkg/buildinfo"
)

func NewCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "version",
		Short: "Print the ark version and associated image",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Version: %s\n", buildinfo.Version)
			fmt.Printf("Git commit: %s\n", buildinfo.GitSHA)
			fmt.Printf("Git tree state: %s\n", buildinfo.GitTreeState)
		},
	}

	return c
}
