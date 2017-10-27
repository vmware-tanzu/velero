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

package ark

import (
	"flag"

	"github.com/spf13/cobra"

	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cmd/cli/backup"
	"github.com/heptio/ark/pkg/cmd/cli/create"
	"github.com/heptio/ark/pkg/cmd/cli/get"
	"github.com/heptio/ark/pkg/cmd/cli/restore"
	"github.com/heptio/ark/pkg/cmd/cli/schedule"
	"github.com/heptio/ark/pkg/cmd/server"
	"github.com/heptio/ark/pkg/cmd/version"
)

func NewCommand(name string) *cobra.Command {
	c := &cobra.Command{
		Use:   name,
		Short: "Back up and restore Kubernetes cluster resources.",
		Long: `Heptio Ark is a tool for managing disaster recovery, specifically for Kubernetes
cluster resources. It provides a simple, configurable, and operationally robust
way to back up your application state and associated data.

If you're familiar with kubectl, Ark supports a similar model, allowing you to
execute commands such as 'ark get backup' and 'ark create schedule'. The same
operations can also be performed as 'ark backup get' and 'ark schedule create'.`,
	}

	f := client.NewFactory(name)
	f.BindFlags(c.PersistentFlags())

	c.AddCommand(
		backup.NewCommand(f),
		schedule.NewCommand(f),
		restore.NewCommand(f),
		server.NewCommand(),
		version.NewCommand(),
		get.NewCommand(f),
		create.NewCommand(f),
	)

	// add the glog flags
	c.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	// TODO: switch to a different logging library.
	// Work around https://github.com/golang/glog/pull/13.
	// See also https://github.com/kubernetes/kubernetes/issues/17162
	flag.CommandLine.Parse([]string{})

	return c
}
