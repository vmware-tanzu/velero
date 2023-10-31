/*
Copyright The Velero Contributors.

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
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestDualModeLogger(t *testing.T) {
	logMsgExpect := "Expected message in log"
	logMsgUnexpect := "Unexpected message in log"

	logger, err := NewTempFileLogger(logrus.DebugLevel, FormatText, nil, logrus.Fields{})
	require.NoError(t, err)

	logger.Info(logMsgExpect)

	logger.DoneForPersist(velerotest.NewLogger())

	logger.Info(logMsgUnexpect)

	logFile, err := logger.GetPersistFile()
	require.NoError(t, err)

	logStr, err := readLogString(logFile)
	require.NoError(t, err)

	assert.Equal(t, true, strings.Contains(logStr, logMsgExpect))
	assert.Equal(t, false, strings.Contains(logStr, logMsgUnexpect))

	logger.Dispose(velerotest.NewLogger())

	_, err = os.Stat(logFile.Name())

	assert.Equal(t, true, os.IsNotExist(err))
}

func TestDualModeLoggerEntryHook(t *testing.T) {
	logCounter := NewLogHook()
	logger, err := NewTempFileLogger(
		logrus.WarnLevel,
		FormatText,
		logCounter,
		//logrus.Fields{"namespace": "ns1", "resource": "pod", "name": "podxxx"})
		logrus.Fields{"namespace": "ns1", "resource": "podxxxx"})
	require.NoError(t, err)

	logger.Warnln("test 1")
	logger.Warnln("test 2")
	logger.Warnln("test 3")

	logger.WithFields(logrus.Fields{"namespace": "ns2", "resource": "pod", "name": "pod1"})
	logger.Warnln("test 1 with field ns2")
	logger.WithFields(logrus.Fields{"namespace": "ns2", "resource": "pod", "name": "pod2"})
	logger.Warnln("test 2 with field ns2")
	logger.WithFields(logrus.Fields{"namespace": "ns2", "resource": "pod", "name": "pod3"})
	logger.Warnln("test 3 with field ns2")

	logger.WithFields(logrus.Fields{"namespace": "ns3", "resource": "pod", "name": "pod4"})
	logger.Warnln("test 4 with field ns3")
	logger.WithFields(logrus.Fields{"namespace": "ns3", "resource": "pod", "name": "pod5"})
	logger.Warnln("test 5 with field ns3")
	logger.WithFields(logrus.Fields{"namespace": "ns3", "resource": "pod", "name": "pod6"})
	logger.Warnln("test 6 with field ns3")

	logger.WithError(errors.New("found err 1")).WithFields(
		logrus.Fields{"namespace": "ns4", "resource": "pod", "name": "pod7"})
	logger.Warnln("test 7 with field ns4 and WithError")
	logger.WithError(errors.New("found err 2")).WithFields(
		logrus.Fields{"namespace": "ns4", "resource": "pod", "name": "pod8"})
	logger.Warnln("test 8 with field ns4 and WithError")

	logger.Errorln("test error 1 in pod 1")
	logger.Errorln("test error 2 in pod 2")
	logger.Errorln("test error 3 in pod 3")
	logger.Errorln("test error 4 in pod 4")

	logger.DoneForPersist(velerotest.NewLogger())

	logCntForWarn := logCounter.GetCount(logrus.WarnLevel)
	fmt.Printf("logCntForWarn=%v\n", logCntForWarn)

	logCntForError := logCounter.GetCount(logrus.ErrorLevel)
	fmt.Printf("logCntForError=%v\n", logCntForError)

	logResultForWarn := logCounter.GetEntries(logrus.WarnLevel)
	for key, value := range logResultForWarn.Namespaces {
		fmt.Printf("logResultForWarn namespace key=%+v , value=%+v\n", key, value)
	}

	logResultForError := logCounter.GetEntries(logrus.WarnLevel)
	for key, value := range logResultForError.Namespaces {
		fmt.Printf("logResultForError namespace key=%+v , value=%+v\n", key, value)
	}

	logFile, err := logger.GetPersistFile()
	require.NoError(t, err)

	logger.Dispose(velerotest.NewLogger())

	_, err = os.Stat(logFile.Name())
	assert.Equal(t, true, os.IsNotExist(err))
}

func readLogString(file *os.File) (string, error) {
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}

	buffer := make([]byte, 1024)
	_, err = gzr.Read(buffer)
	if err != io.EOF {
		return "", err
	}

	return string(buffer[:]), nil
}
