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

package logging

import (
	"bytes"
	goerrors "errors"
	"regexp"
	"testing"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestLogStackTrace(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		expectedRE string
	}{
		{
			name:       "error without stack",
			err:        goerrors.New("no stack here"),
			expectedRE: "^$",
		},
		{
			name:       "arkerrors.StackTracer",
			err:        errors.Errorf("stack!"),
			expectedRE: "stack_trace_test\\.go:43",
		},
		{
			name:       "stackTraceStringer",
			err:        &testStackTraceStringer{},
			expectedRE: "my stack trace",
		},
	}

	for _, debugLevel := range []bool{true, false} {
		for _, tc := range tests {
			testName := tc.name
			if debugLevel {
				testName += "-debug"
			}
			t.Run(testName, func(t *testing.T) {
				log := logrus.New()
				buf := &bytes.Buffer{}
				log.Out = buf
				if debugLevel {
					log.Level = logrus.DebugLevel
				} else {
					log.Level = logrus.InfoLevel
				}

				LogStackTrace(log, tc.err)

				switch {
				case !debugLevel:
					assert.Empty(t, buf.String())
				case tc.expectedRE != "":
					re := regexp.MustCompile(tc.expectedRE)
					t.Logf("stack: %s", buf.String())
					assert.True(t, re.MatchString(buf.String()))
				}
			})
		}
	}
}

type testStackTraceStringer struct{}

func (s *testStackTraceStringer) StackTrace() string {
	return "my stack trace"
}

func (s *testStackTraceStringer) Error() string {
	return "some error"
}
