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

package logging

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

const logLocationField = "logSource"

// LogLocationHook is a logrus hook that attaches location information
// to log entries, i.e. the file and line number of the logrus log call.
type LogLocationHook struct {
}

func (h *LogLocationHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *LogLocationHook) Fire(entry *logrus.Entry) error {
	pcs := make([]uintptr, 64)

	// skip 2 frames:
	//   runtime.Callers
	//   github.com/heptio/ark/pkg/util/logging/(*LogLocationHook).Fire
	n := runtime.Callers(2, pcs)

	// re-slice pcs based on the number of entries written
	frames := runtime.CallersFrames(pcs[:n])

	// now traverse up the call stack looking for the first non-logrus
	// func, which will be the logrus invoker
	var (
		frame runtime.Frame
		more  = true
	)

	for more {
		frame, more = frames.Next()

		if strings.Contains(frame.File, "github.com/sirupsen/logrus") {
			continue
		}

		entry.Data[logLocationField] = fmt.Sprintf("%s:%d", frame.File, frame.Line)
		break
	}

	return nil
}
