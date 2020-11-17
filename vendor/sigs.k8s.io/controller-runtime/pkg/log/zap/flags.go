/*
Copyright 2020 The Kubernetes Authors.

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

// Package zap contains helpers for setting up a new logr.Logger instance
// using the Zap logging framework.
package zap

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var levelStrings = map[string]zapcore.Level{
	"debug":  zap.DebugLevel,
	"-1":     zap.DebugLevel,
	"info":   zap.InfoLevel,
	"0":      zap.InfoLevel,
	"error":  zap.ErrorLevel,
	"2":      zap.ErrorLevel,
	"dpanic": zap.DPanicLevel,
	"panic":  zap.PanicLevel,
	"warn":   zap.WarnLevel,
	"fatal":  zap.FatalLevel,
}

type encoderFlag struct {
	setFunc func(zapcore.Encoder)
	value   string
}

var _ pflag.Value = &encoderFlag{}

func (ev *encoderFlag) String() string {
	return ev.value
}

func (ev *encoderFlag) Type() string {
	return "encoder"
}

func (ev *encoderFlag) Set(flagValue string) error {
	val := strings.ToLower(flagValue)
	switch val {
	case "json":
		ev.setFunc(newJSONEncoder())
	case "console":
		ev.setFunc(newConsoleEncoder())
	default:
		return fmt.Errorf("invalid encoder value \"%s\"", flagValue)
	}
	ev.value = flagValue
	return nil
}

func newJSONEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	return zapcore.NewJSONEncoder(encoderConfig)
}

func newConsoleEncoder() zapcore.Encoder {
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	return zapcore.NewConsoleEncoder(encoderConfig)
}

type levelFlag struct {
	setFunc func(zapcore.LevelEnabler)
	value   string
}

var _ pflag.Value = &levelFlag{}

func (ev *levelFlag) Set(flagValue string) error {
	level, validLevel := levelStrings[strings.ToLower(flagValue)]
	if !validLevel {
		logLevel, err := strconv.Atoi(flagValue)
		if err != nil {
			return fmt.Errorf("invalid log level \"%s\"", flagValue)
		}
		if logLevel > 0 {
			intLevel := -1 * logLevel
			ev.setFunc(zap.NewAtomicLevelAt(zapcore.Level(int8(intLevel))))
		} else {
			return fmt.Errorf("invalid log level \"%s\"", flagValue)
		}
	}
	ev.setFunc(zap.NewAtomicLevelAt(level))
	ev.value = flagValue
	return nil
}

func (ev *levelFlag) String() string {
	return ev.value
}

func (ev *levelFlag) Type() string {
	return "level"
}

type stackTraceFlag struct {
	setFunc func(zapcore.LevelEnabler)
	value   string
}

var _ pflag.Value = &stackTraceFlag{}

func (ev *stackTraceFlag) Set(flagValue string) error {
	level, validLevel := levelStrings[strings.ToLower(flagValue)]
	if !validLevel {
		return fmt.Errorf("invalid stacktrace level \"%s\"", flagValue)
	}
	ev.setFunc(zap.NewAtomicLevelAt(level))
	ev.value = flagValue
	return nil
}

func (ev *stackTraceFlag) String() string {
	return ev.value
}

func (ev *stackTraceFlag) Type() string {
	return "level"
}
