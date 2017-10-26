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

package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cmd"
	"github.com/heptio/ark/pkg/cmd/util/downloadrequest"
)

func NewDownloadCommand(f client.Factory) *cobra.Command {
	o := NewDownloadOptions()
	c := &cobra.Command{
		Use:   "download NAME",
		Short: "Download a backup",
		Run: func(c *cobra.Command, args []string) {
			cmd.CheckError(o.Validate(c, args))
			cmd.CheckError(o.Complete(args))
			cmd.CheckError(o.Run(c, f))
		},
	}

	o.BindFlags(c.Flags())

	return c
}

type DownloadOptions struct {
	Name         string
	Output       string
	Force        bool
	Timeout      time.Duration
	writeOptions int
}

func NewDownloadOptions() *DownloadOptions {
	return &DownloadOptions{
		Timeout: time.Minute,
	}
}

func (o *DownloadOptions) BindFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&o.Output, "output", "o", o.Output, "path to output file. Defaults to <NAME>-data.tar.gz in the current directory")
	flags.BoolVar(&o.Force, "force", o.Force, "forces the download and will overwrite file if it exists already")
	flags.DurationVar(&o.Timeout, "timeout", o.Timeout, "maximum time to wait to process download request")
}

func (o *DownloadOptions) Validate(c *cobra.Command, args []string) error {
	if len(args) != 1 {
		return errors.New("backup name is required")
	}

	return nil
}

func (o *DownloadOptions) Complete(args []string) error {
	o.Name = args[0]

	o.writeOptions = os.O_RDWR | os.O_CREATE | os.O_EXCL
	if o.Force {
		o.writeOptions = os.O_RDWR | os.O_CREATE | os.O_TRUNC
	}

	if o.Output == "" {
		path, err := os.Getwd()
		if err != nil {
			return errors.Wrapf(err, "error getting current directory")
		}
		o.Output = filepath.Join(path, fmt.Sprintf("%s-data.tar.gz", o.Name))
	}

	return nil
}

func (o *DownloadOptions) Run(c *cobra.Command, f client.Factory) error {
	arkClient, err := f.Client()
	cmd.CheckError(err)

	backupDest, err := os.OpenFile(o.Output, o.writeOptions, 0600)
	if err != nil {
		return err
	}
	defer backupDest.Close()

	err = downloadrequest.Stream(arkClient.ArkV1(), o.Name, v1.DownloadTargetKindBackupContents, backupDest, o.Timeout)
	if err != nil {
		os.Remove(o.Output)
		cmd.CheckError(err)
	}

	fmt.Printf("Backup %s has been successfully downloaded to %s\n", o.Name, backupDest.Name())
	return nil
}
