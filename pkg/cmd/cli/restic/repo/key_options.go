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

package repo

import (
	"crypto/rand"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"

	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/util/filesystem"
)

type RepositoryKeyOptions struct {
	KeyFile string
	KeyData string
	KeySize int

	fileSystem filesystem.Interface
	keyBytes   []byte
}

func NewRepositoryKeyOptions() *RepositoryKeyOptions {
	return &RepositoryKeyOptions{
		KeySize:    1024,
		fileSystem: filesystem.NewFileSystem(),
	}
}

var (
	errKeyFileAndKeyDataProvided = errors.Errorf("only one of --key-file and --key-data may be specified")
	errKeySizeTooSmall           = errors.Errorf("--key-size must be at least 1")
)

func (o *RepositoryKeyOptions) BindFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.KeyFile, "key-file", o.KeyFile, "Path to file containing the encryption key for the restic repository. Optional; if unset, Ark will generate a random key for you.")
	flags.StringVar(&o.KeyData, "key-data", o.KeyData, "Encryption key for the restic repository. Optional; if unset, Ark will generate a random key for you.")
	flags.IntVar(&o.KeySize, "key-size", o.KeySize, "Size of the generated key for the restic repository")
}

func (o *RepositoryKeyOptions) Complete(f client.Factory, args []string) error {
	if o.KeyFile != "" && o.KeyData != "" {
		return errKeyFileAndKeyDataProvided
	}

	if o.KeyFile == "" && o.KeyData == "" && o.KeySize < 1 {
		return errKeySizeTooSmall
	}

	switch {
	case o.KeyFile != "":
		data, err := o.fileSystem.ReadFile(o.KeyFile)
		if err != nil {
			return err
		}
		o.keyBytes = data
	case o.KeyData != "":
		o.keyBytes = []byte(o.KeyData)
	case o.KeySize > 0:
		o.keyBytes = make([]byte, o.KeySize)
		// rand.Reader always returns a nil error
		rand.Read(o.keyBytes)
	}

	return nil
}

func (o *RepositoryKeyOptions) Validate(f client.Factory) error {
	if len(o.keyBytes) == 0 {
		return errors.Errorf("keyBytes is required")
	}

	return nil
}
