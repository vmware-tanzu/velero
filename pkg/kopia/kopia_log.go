package kopia

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

import (
	"context"

	"github.com/kopia/kopia/repo/logging"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type kopiaLog struct {
	module string
	logger logrus.FieldLogger
}

// SetupKopiaLog sets the Kopia log handler to the specific context, Kopia modules
// call the logger in the context to write logs
func SetupKopiaLog(ctx context.Context, logger logrus.FieldLogger) context.Context {
	return logging.WithLogger(ctx, func(module string) logging.Logger {
		kpLog := &kopiaLog{module, logger}
		return zap.New(kpLog).Sugar()
	})
}

// Enabled decides whether a given logging level is enabled when logging a message
func (kl *kopiaLog) Enabled(level zapcore.Level) bool {
	entry := kl.logger.WithField("null", "null")
	switch level {
	case zapcore.DebugLevel:
		return (entry.Logger.GetLevel() >= logrus.DebugLevel)
	case zapcore.InfoLevel:
		return (entry.Logger.GetLevel() >= logrus.InfoLevel)
	case zapcore.WarnLevel:
		return (entry.Logger.GetLevel() >= logrus.WarnLevel)
	case zapcore.ErrorLevel:
		return (entry.Logger.GetLevel() >= logrus.ErrorLevel)
	case zapcore.DPanicLevel:
		return (entry.Logger.GetLevel() >= logrus.PanicLevel)
	case zapcore.PanicLevel:
		return (entry.Logger.GetLevel() >= logrus.PanicLevel)
	case zapcore.FatalLevel:
		return (entry.Logger.GetLevel() >= logrus.FatalLevel)
	default:
		return false
	}
}

// With adds structured context to the Core.
func (kl *kopiaLog) With(fields []zapcore.Field) zapcore.Core {
	copied := kl.logrusFields(fields)

	return &kopiaLog{
		module: kl.module,
		logger: kl.logger.WithFields(copied),
	}
}

// Check determines whether the supplied Entry should be logged. If the entry
// should be logged, the Core adds itself to the CheckedEntry and returns the result.
func (kl *kopiaLog) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if kl.Enabled(ent.Level) {
		return ce.AddCore(ent, kl)
	}

	return ce
}

// Write serializes the Entry and any Fields supplied at the log site and writes them to their destination.
func (kl *kopiaLog) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	copied := kl.logrusFieldsForWrite(ent, fields)
	logger := kl.logger.WithFields(copied)

	switch ent.Level {
	case zapcore.DebugLevel:
		logger.Debug(ent.Message)
	case zapcore.InfoLevel:
		logger.Info(ent.Message)
	case zapcore.WarnLevel:
		logger.Warn(ent.Message)
	case zapcore.ErrorLevel:
		// We see Kopia generates error logs for some normal cases or non-critical
		// cases. So Kopia's error logs are regarded as warning logs so that they don't
		// affect Velero's workflow.
		logger.Warn(ent.Message)
	case zapcore.DPanicLevel:
		logger.Panic(ent.Message)
	case zapcore.PanicLevel:
		logger.Panic(ent.Message)
	case zapcore.FatalLevel:
		logger.Fatal(ent.Message)
	}

	return nil
}

// Sync flushes buffered logs (if any).
func (kl *kopiaLog) Sync() error {
	return nil
}

func (kl *kopiaLog) logrusFields(fields []zapcore.Field) logrus.Fields {
	if fields == nil {
		return logrus.Fields{}
	}

	m := zapcore.NewMapObjectEncoder()
	for _, field := range fields {
		field.AddTo(m)
	}

	return m.Fields
}

func (kl *kopiaLog) getLogModule() string {
	return "kopia/" + kl.module
}

func (kl *kopiaLog) logrusFieldsForWrite(ent zapcore.Entry, fields []zapcore.Field) logrus.Fields {
	copied := kl.logrusFields(fields)

	copied["logModule"] = kl.getLogModule()

	if ent.Caller.Function != "" {
		copied["function"] = ent.Caller.Function
	}

	path := ent.Caller.FullPath()
	if path != "undefined" {
		copied["path"] = path
	}

	if ent.LoggerName != "" {
		copied["logger name"] = ent.LoggerName
	}

	if ent.Stack != "" {
		copied["stack"] = ent.Stack
	}

	if ent.Level == zap.ErrorLevel {
		copied["sublevel"] = "error"
	}

	return copied
}
