/*
Copyright 2018 the Velero contributors.

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

package logging

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestDefaultLogger(t *testing.T) {
	formatFlag := NewFormatFlag()

	for _, testFormat := range formatFlag.AllowedValues() {
		formatFlag.Set(testFormat)
		logger := DefaultLogger(logrus.InfoLevel, formatFlag.Parse())
		assert.Equal(t, logrus.InfoLevel, logger.Level)
		assert.Equal(t, os.Stdout, logger.Out)

		for _, level := range logrus.AllLevels {
			assert.Equal(t, DefaultHooks(false), logger.Hooks[level])
		}
	}
}

func TestDefaultMergeLogger(t *testing.T) {
	formatFlag := NewFormatFlag()

	for _, testFormat := range formatFlag.AllowedValues() {
		formatFlag.Set(testFormat)
		logger := DefaultMergeLogger(logrus.InfoLevel, formatFlag.Parse())
		assert.Equal(t, logrus.InfoLevel, logger.Level)
		assert.Equal(t, os.Stdout, logger.Out)

		assert.Equal(t, DefaultHooks(true), logger.Hooks[ListeningLevel])
	}
}
