/*
Copyright 2017 the Heptio Ark contributors.

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
	"github.com/sirupsen/logrus"
)

// HcLogLevelHook adds an hclog-compatible field ("@level") containing
// the log level. Note that if you use this, you SHOULD NOT use
// logrus.JSONFormatter's FieldMap to set the level key to "@level" because
// that will result in the hclog-compatible info written here being
// overwritten.
type HcLogLevelHook struct{}

func (h *HcLogLevelHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *HcLogLevelHook) Fire(entry *logrus.Entry) error {
	switch entry.Level {
	// logrus uses "warning" to represent WarnLevel,
	// which is not compatible with hclog's "warn".
	case logrus.WarnLevel:
		entry.Data["@level"] = "warn"
	default:
		entry.Data["@level"] = entry.Level.String()
	}

	return nil
}
