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

package client

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

const (
	ConfigKeyNamespace = "namespace"
)

// LoadConfig loads the Ark client configuration file and returns it as a map[string]string. If the
// file does not exist, an empty map is returned.
func LoadConfig() (map[string]string, error) {
	fileName := configFileName()

	_, err := os.Stat(fileName)
	if os.IsNotExist(err) {
		// If the file isn't there, just return an empty map
		return map[string]string{}, nil
	}
	if err != nil {
		// For any other Stat() error, return it
		return nil, errors.WithStack(err)
	}

	configFile, err := os.Open(fileName)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer configFile.Close()

	var config map[string]string
	if err := json.NewDecoder(configFile).Decode(&config); err != nil {
		return nil, errors.WithStack(err)
	}

	return config, nil
}

// SaveConfig saves the passed in config map to the Ark client configuration file.
func SaveConfig(config map[string]string) error {
	fileName := configFileName()

	// Try to make the directory in case it doesn't exist
	dir := filepath.Dir(fileName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.WithStack(err)
	}

	configFile, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return errors.WithStack(err)
	}
	defer configFile.Close()

	return json.NewEncoder(configFile).Encode(&config)
}

func configFileName() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "ark", "config.json")
}
