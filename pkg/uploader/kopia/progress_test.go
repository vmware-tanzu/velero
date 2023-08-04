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

package kopia

import (
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/vmware-tanzu/velero/pkg/uploader"
)

type fakeProgressUpdater struct{}

func (f *fakeProgressUpdater) UpdateProgress(p *uploader.Progress) {}

func TestThrottle_ShouldOutput(t *testing.T) {
	testCases := []struct {
		interval       time.Duration
		throttle       int64
		expectedOutput bool
	}{
		{interval: time.Second, expectedOutput: true},
		{interval: time.Second, throttle: time.Now().UnixNano() + int64(time.Nanosecond*100000000), expectedOutput: false},
	}
	p := new(Progress)
	for _, tc := range testCases {
		// Setup
		p.InitThrottle(tc.interval)
		p.outputThrottle.throttle = int64(tc.throttle)
		// Perform the test

		output := p.outputThrottle.ShouldOutput()

		// Verify the result
		if output != tc.expectedOutput {
			t.Errorf("Expected ShouldOutput to return %v, but got %v", tc.expectedOutput, output)
		}
	}
}

func TestProgress(t *testing.T) {
	fileName := "test-filename"
	var numBytes int64 = 1
	testCases := []struct {
		interval time.Duration
		throttle int64
	}{
		{interval: time.Second},
		{interval: time.Second, throttle: time.Now().UnixNano() + int64(time.Nanosecond*10000)},
	}
	p := new(Progress)
	p.Log = logrus.New()
	p.Updater = &fakeProgressUpdater{}
	for _, tc := range testCases {
		// Setup
		p.InitThrottle(tc.interval)
		p.outputThrottle.throttle = int64(tc.throttle)
		p.InitThrottle(time.Duration(time.Second))
		// All below calls put together for the implementation are empty or just very simple and just want to cover testing
		// If wanting to write unit tests for some functions could remove it and with writing new function alone
		p.UpdateProgress()
		p.UploadedBytes(numBytes)
		p.Error("test-path", nil, true)
		p.Error("test-path", errors.New("processing error"), false)
		p.UploadStarted()
		p.EstimatedDataSize(1, numBytes)
		p.CachedFile(fileName, numBytes)
		p.HashedBytes(numBytes)
		p.HashingFile(fileName)
		p.ExcludedFile(fileName, numBytes)
		p.ExcludedDir(fileName)
		p.FinishedHashingFile(fileName, numBytes)
		p.StartedDirectory(fileName)
		p.FinishedDirectory(fileName)
		p.UploadFinished()
		p.ProgressBytes(numBytes, numBytes)
		p.FinishedFile(fileName, nil)
	}
}
