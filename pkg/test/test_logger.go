/*
Copyright 2017 the Velero contributors.

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

package test

import (
	"io"

	"github.com/sirupsen/logrus"
)

func NewLogger() logrus.FieldLogger {
	logger := logrus.New()
	logger.Out = io.Discard
	return logrus.NewEntry(logger)
}

func NewLoggerWithLevel(level logrus.Level) logrus.FieldLogger {
	logger := logrus.New()
	logger.Out = io.Discard
	logger.Level = level
	return logrus.NewEntry(logger)
}

type singleLogRecorder struct {
	buffer *string
}

func (s *singleLogRecorder) Write(p []byte) (n int, err error) {
	*s.buffer = *s.buffer + string(p[:])
	return len(p), nil
}

func NewSingleLogger(buffer *string) logrus.FieldLogger {
	logger := logrus.New()
	logger.Out = &singleLogRecorder{buffer: buffer}
	logger.Level = logrus.TraceLevel
	return logrus.NewEntry(logger)
}

func NewSingleLoggerWithHooks(buffer *string, hooks []logrus.Hook) logrus.FieldLogger {
	logger := logrus.New()
	logger.Out = &singleLogRecorder{buffer: buffer}
	logger.Level = logrus.TraceLevel

	for _, hook := range hooks {
		logger.Hooks.Add(hook)
	}

	return logrus.NewEntry(logger)
}

type multipleLogRecorder struct {
	buffer *[]string
}

func (m *multipleLogRecorder) Write(p []byte) (n int, err error) {
	*m.buffer = append(*m.buffer, string(p[:]))
	return len(p), nil
}

func NewMultipleLogger(buffer *[]string) logrus.FieldLogger {
	logger := logrus.New()
	logger.Out = &multipleLogRecorder{buffer}
	logger.Level = logrus.TraceLevel
	return logrus.NewEntry(logger)
}
