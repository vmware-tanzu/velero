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

	"github.com/sirupsen/logrus"
)

type ctxKeyLogger struct{}

// WithLogger returns a new context with the provided logger.
func WithLogger(ctx context.Context, logger logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, ctxKeyLogger{}, logger)
}

// LoggerFromContext retrieves the logger from the context, or returns a default logger if none found.
func LoggerFromContext(ctx context.Context) logrus.FieldLogger {
	if logger, ok := ctx.Value(ctxKeyLogger{}).(logrus.FieldLogger); ok && logger != nil {
		return logger
	}
	return logrus.New()
}
