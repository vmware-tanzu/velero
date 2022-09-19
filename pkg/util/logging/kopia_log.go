/*
Copyright the Velero contributors.

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
	"context"

	"github.com/kopia/kopia/repo/logging"
	"github.com/sirupsen/logrus"
)

type kopiaLog struct {
	module string
	logger logrus.FieldLogger
}

// SetupKopiaLog sets the Kopia log handler to the specific context, Kopia modules
// call the logger in the context to write logs
func SetupKopiaLog(ctx context.Context, logger logrus.FieldLogger) context.Context {
	return logging.WithLogger(ctx, func(module string) logging.Logger {
		return &kopiaLog{
			module: module,
			logger: logger,
		}
	})
}

func (kl *kopiaLog) Debugf(msg string, args ...interface{}) {
	logger := kl.logger.WithField("logModule", kl.getLogModule())
	logger.Debugf(msg, args...)
}

func (kl *kopiaLog) Debugw(msg string, keyValuePairs ...interface{}) {
	logger := kl.logger.WithField("logModule", kl.getLogModule())
	logger.WithFields(getLogFields(keyValuePairs...)).Debug(msg)
}

func (kl *kopiaLog) Infof(msg string, args ...interface{}) {
	logger := kl.logger.WithField("logModule", kl.getLogModule())
	logger.Infof(msg, args...)
}

func (kl *kopiaLog) Warnf(msg string, args ...interface{}) {
	logger := kl.logger.WithField("logModule", kl.getLogModule())
	logger.Warnf(msg, args...)
}

// We see Kopia generates error logs for some normal cases or non-critical
// cases. So Kopia's error logs are regarded as warning logs so that they don't
// affect Velero's workflow.
func (kl *kopiaLog) Errorf(msg string, args ...interface{}) {
	logger := kl.logger.WithFields(logrus.Fields{
		"logModule": kl.getLogModule(),
		"sublevel":  "error",
	})

	logger.Warnf(msg, args...)
}

func (kl *kopiaLog) getLogModule() string {
	return "kopia/" + kl.module
}

func getLogFields(keyValuePairs ...interface{}) map[string]interface{} {
	m := map[string]interface{}{}
	for i := 0; i+1 < len(keyValuePairs); i += 2 {
		s, ok := keyValuePairs[i].(string)
		if !ok {
			s = "non-string-key"
		}

		m[s] = keyValuePairs[i+1]
	}

	return m
}
