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

package datapath

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	kubeclientfake "k8s.io/client-go/kubernetes/fake"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/vmware-tanzu/velero/pkg/builder"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/uploader"
	"github.com/vmware-tanzu/velero/pkg/util/logging"
)

func TestReEnsureThisPod(t *testing.T) {
	tests := []struct {
		name          string
		namespace     string
		thisPod       string
		kubeClientObj []runtime.Object
		expectChan    bool
		expectErr     string
	}{
		{
			name:      "get pod error",
			thisPod:   "fak-pod-1",
			expectErr: "error getting this pod fak-pod-1: pods \"fak-pod-1\" not found",
		},
		{
			name:      "get pod not in terminated state",
			namespace: "velero",
			thisPod:   "fake-pod-1",
			kubeClientObj: []runtime.Object{
				builder.ForPod("velero", "fake-pod-1").Phase(v1.PodRunning).Result(),
			},
		},
		{
			name:      "get pod succeed state",
			namespace: "velero",
			thisPod:   "fake-pod-1",
			kubeClientObj: []runtime.Object{
				builder.ForPod("velero", "fake-pod-1").Phase(v1.PodSucceeded).Result(),
			},
			expectChan: true,
		},
		{
			name:      "get pod failed state",
			namespace: "velero",
			thisPod:   "fake-pod-1",
			kubeClientObj: []runtime.Object{
				builder.ForPod("velero", "fake-pod-1").Phase(v1.PodFailed).Result(),
			},
			expectChan: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			v1.AddToScheme(scheme)
			fakeClientBuilder := fake.NewClientBuilder()
			fakeClientBuilder = fakeClientBuilder.WithScheme(scheme)

			fakeClient := fakeClientBuilder.WithRuntimeObjects(test.kubeClientObj...).Build()

			ms := &microServiceBRWatcher{
				namespace: test.namespace,
				thisPod:   test.thisPod,
				client:    fakeClient,
				podCh:     make(chan *v1.Pod, 2),
				log:       velerotest.NewLogger(),
			}

			err := ms.reEnsureThisPod(context.Background())
			if test.expectErr != "" {
				assert.EqualError(t, err, test.expectErr)
			} else {
				if test.expectChan {
					assert.Len(t, ms.podCh, 1)
					pod := <-ms.podCh
					assert.Equal(t, pod.Name, test.thisPod)
				}
			}
		})
	}
}

type startWatchFake struct {
	terminationMessage string
	redirectErr        error
	complete           bool
	failed             bool
	canceled           bool
	progress           int
}

func (sw *startWatchFake) getPodContainerTerminateMessage(_ *v1.Pod, _ string) string {
	return sw.terminationMessage
}

func (sw *startWatchFake) redirectDataMoverLogs(_ context.Context, _ kubernetes.Interface, _ string, _ string, _ string, _ logrus.FieldLogger) error {
	return sw.redirectErr
}

func (sw *startWatchFake) getResultFromMessage(_ string, _ string, _ logrus.FieldLogger) Result {
	return Result{}
}

func (sw *startWatchFake) OnCompleted(_ context.Context, _ string, _ string, _ Result) {
	sw.complete = true
}

func (sw *startWatchFake) OnFailed(_ context.Context, _ string, _ string, _ error) {
	sw.failed = true
}

func (sw *startWatchFake) OnCancelled(_ context.Context, _ string, _ string) {
	sw.canceled = true
}

func (sw *startWatchFake) OnProgress(_ context.Context, _ string, _ string, _ *uploader.Progress) {
	sw.progress++
}

type insertEvent struct {
	event *v1.Event
	after time.Duration
	delay time.Duration
}

func TestStartWatch(t *testing.T) {
	tests := []struct {
		name                 string
		namespace            string
		thisPod              string
		thisContainer        string
		terminationMessage   string
		redirectLogErr       error
		insertPod            *v1.Pod
		insertEventsBefore   []insertEvent
		insertEventsAfter    []insertEvent
		ctxCancel            bool
		expectStartEvent     bool
		expectTerminateEvent bool
		expectComplete       bool
		expectCancel         bool
		expectFail           bool
		expectProgress       int
	}{
		{
			name:          "exit from ctx",
			thisPod:       "fak-pod-1",
			thisContainer: "fake-container-1",
			ctxCancel:     true,
		},
		{
			name:          "completed with rantional sequence",
			thisPod:       "fak-pod-1",
			thisContainer: "fake-container-1",
			insertPod:     builder.ForPod("velero", "fake-pod-1").Phase(v1.PodSucceeded).Result(),
			insertEventsBefore: []insertEvent{
				{
					event: &v1.Event{Reason: EventReasonStarted},
				},
				{
					event: &v1.Event{Reason: EventReasonCompleted},
				},
				{
					event: &v1.Event{Reason: EventReasonStopped},
					delay: time.Second,
				},
			},
			expectStartEvent:     true,
			expectTerminateEvent: true,
			expectComplete:       true,
			expectProgress:       1,
		},
		{
			name:          "completed",
			thisPod:       "fak-pod-1",
			thisContainer: "fake-container-1",
			insertPod:     builder.ForPod("velero", "fake-pod-1").Phase(v1.PodSucceeded).Result(),
			insertEventsBefore: []insertEvent{
				{
					event: &v1.Event{Reason: EventReasonStarted},
				},
				{
					event: &v1.Event{Reason: EventReasonCompleted},
				},
				{
					event: &v1.Event{Reason: EventReasonStopped},
				},
			},
			expectStartEvent:     true,
			expectTerminateEvent: true,
			expectComplete:       true,
			expectProgress:       1,
		},
		{
			name:          "completed with redirect error",
			thisPod:       "fak-pod-1",
			thisContainer: "fake-container-1",
			insertPod:     builder.ForPod("velero", "fake-pod-1").Phase(v1.PodSucceeded).Result(),
			insertEventsBefore: []insertEvent{
				{
					event: &v1.Event{Reason: EventReasonStarted},
				},
				{
					event: &v1.Event{Reason: EventReasonCompleted},
				},
				{
					event: &v1.Event{Reason: EventReasonStopped},
				},
			},
			redirectLogErr:       errors.New("fake-error"),
			expectStartEvent:     true,
			expectTerminateEvent: true,
			expectComplete:       true,
			expectProgress:       1,
		},
		{
			name:          "complete but terminated event not received in time",
			thisPod:       "fak-pod-1",
			thisContainer: "fake-container-1",
			insertPod:     builder.ForPod("velero", "fake-pod-1").Phase(v1.PodSucceeded).Result(),
			insertEventsBefore: []insertEvent{
				{
					event: &v1.Event{Reason: EventReasonStarted},
				},
			},
			insertEventsAfter: []insertEvent{
				{
					event: &v1.Event{Reason: EventReasonStarted},
					after: time.Second * 6,
				},
			},
			expectStartEvent: true,
			expectComplete:   true,
			expectProgress:   1,
		},
		{
			name:          "complete but terminated event not received immediately",
			thisPod:       "fak-pod-1",
			thisContainer: "fake-container-1",
			insertPod:     builder.ForPod("velero", "fake-pod-1").Phase(v1.PodSucceeded).Result(),
			insertEventsBefore: []insertEvent{
				{
					event: &v1.Event{Reason: EventReasonStarted},
				},
			},
			insertEventsAfter: []insertEvent{
				{
					event: &v1.Event{Reason: EventReasonCompleted},
				},
				{
					event: &v1.Event{Reason: EventReasonStopped},
					delay: time.Second,
				},
			},
			expectStartEvent:     true,
			expectTerminateEvent: true,
			expectComplete:       true,
			expectProgress:       1,
		},
		{
			name:          "completed with progress",
			thisPod:       "fak-pod-1",
			thisContainer: "fake-container-1",
			insertPod:     builder.ForPod("velero", "fake-pod-1").Phase(v1.PodSucceeded).Result(),
			insertEventsBefore: []insertEvent{
				{
					event: &v1.Event{Reason: EventReasonStarted},
				},
				{
					event: &v1.Event{Reason: EventReasonProgress, Message: "fake-progress-1"},
				},
				{
					event: &v1.Event{Reason: EventReasonProgress, Message: "fake-progress-2"},
				},
				{
					event: &v1.Event{Reason: EventReasonCompleted},
				},
				{
					event: &v1.Event{Reason: EventReasonStopped},
					delay: time.Second,
				},
			},
			expectStartEvent:     true,
			expectTerminateEvent: true,
			expectComplete:       true,
			expectProgress:       3,
		},
		{
			name:          "failed",
			thisPod:       "fak-pod-1",
			thisContainer: "fake-container-1",
			insertPod:     builder.ForPod("velero", "fake-pod-1").Phase(v1.PodFailed).Result(),
			insertEventsBefore: []insertEvent{
				{
					event: &v1.Event{Reason: EventReasonStarted},
				},
				{
					event: &v1.Event{Reason: EventReasonCancelled},
				},
				{
					event: &v1.Event{Reason: EventReasonStopped},
				},
			},
			terminationMessage:   "fake-termination-message-1",
			expectStartEvent:     true,
			expectTerminateEvent: true,
			expectFail:           true,
		},
		{
			name:               "pod crash",
			thisPod:            "fak-pod-1",
			thisContainer:      "fake-container-1",
			insertPod:          builder.ForPod("velero", "fake-pod-1").Phase(v1.PodFailed).Result(),
			terminationMessage: "fake-termination-message-2",
			expectFail:         true,
		},
		{
			name:          "canceled",
			thisPod:       "fak-pod-1",
			thisContainer: "fake-container-1",
			insertPod:     builder.ForPod("velero", "fake-pod-1").Phase(v1.PodFailed).Result(),
			insertEventsBefore: []insertEvent{
				{
					event: &v1.Event{Reason: EventReasonStarted},
				},
				{
					event: &v1.Event{Reason: EventReasonCancelled},
				},
				{
					event: &v1.Event{Reason: EventReasonStopped},
				},
			},
			terminationMessage:   fmt.Sprintf("Failed to init data path service for DataUpload %s: %v", "fake-du-name", errors.New(ErrCancelled)),
			expectStartEvent:     true,
			expectTerminateEvent: true,
			expectCancel:         true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			eventWaitTimeout = time.Second * 5

			sw := startWatchFake{
				terminationMessage: test.terminationMessage,
				redirectErr:        test.redirectLogErr,
			}
			funcGetPodTerminationMessage = sw.getPodContainerTerminateMessage
			funcRedirectLog = sw.redirectDataMoverLogs
			funcGetResultFromMessage = sw.getResultFromMessage

			ms := &microServiceBRWatcher{
				ctx:           ctx,
				namespace:     test.namespace,
				thisPod:       test.thisPod,
				thisContainer: test.thisContainer,
				podCh:         make(chan *v1.Pod, 2),
				eventCh:       make(chan *v1.Event, 10),
				log:           velerotest.NewLogger(),
				callbacks: Callbacks{
					OnCompleted: sw.OnCompleted,
					OnFailed:    sw.OnFailed,
					OnCancelled: sw.OnCancelled,
					OnProgress:  sw.OnProgress,
				},
			}

			ms.startWatch()

			if test.ctxCancel {
				cancel()
			}

			for _, ev := range test.insertEventsBefore {
				if ev.after != 0 {
					time.Sleep(ev.after)
				}

				ms.eventCh <- ev.event

				if ev.delay != 0 {
					time.Sleep(ev.delay)
				}
			}

			if test.insertPod != nil {
				ms.podCh <- test.insertPod
			}

			for _, ev := range test.insertEventsAfter {
				if ev.after != 0 {
					time.Sleep(ev.after)
				}

				ms.eventCh <- ev.event

				if ev.delay != 0 {
					time.Sleep(ev.delay)
				}
			}

			ms.wgWatcher.Wait()

			assert.Equal(t, test.expectStartEvent, ms.startedFromEvent)
			assert.Equal(t, test.expectTerminateEvent, ms.terminatedFromEvent)
			assert.Equal(t, test.expectComplete, sw.complete)
			assert.Equal(t, test.expectCancel, sw.canceled)
			assert.Equal(t, test.expectFail, sw.failed)
			assert.Equal(t, test.expectProgress, sw.progress)

			cancel()
		})
	}
}

func TestGetResultFromMessage(t *testing.T) {
	tests := []struct {
		name         string
		taskType     string
		message      string
		expectResult Result
	}{
		{
			name:         "error to unmarshall backup result",
			taskType:     TaskTypeBackup,
			message:      "fake-message",
			expectResult: Result{},
		},
		{
			name:         "error to unmarshall restore result",
			taskType:     TaskTypeRestore,
			message:      "fake-message",
			expectResult: Result{},
		},
		{
			name:     "succeed to unmarshall backup result",
			taskType: TaskTypeBackup,
			message:  "{\"snapshotID\":\"fake-snapshot-id\",\"emptySnapshot\":true,\"source\":{\"byPath\":\"fake-path-1\",\"volumeMode\":\"Block\"}}",
			expectResult: Result{
				Backup: BackupResult{
					SnapshotID:    "fake-snapshot-id",
					EmptySnapshot: true,
					Source: AccessPoint{
						ByPath:  "fake-path-1",
						VolMode: uploader.PersistentVolumeBlock,
					},
				},
			},
		},
		{
			name:     "succeed to unmarshall restore result",
			taskType: TaskTypeRestore,
			message:  "{\"target\":{\"byPath\":\"fake-path-2\",\"volumeMode\":\"Filesystem\"}}",
			expectResult: Result{
				Restore: RestoreResult{
					Target: AccessPoint{
						ByPath:  "fake-path-2",
						VolMode: uploader.PersistentVolumeFilesystem,
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := getResultFromMessage(test.taskType, test.message, velerotest.NewLogger())
			assert.Equal(t, test.expectResult, result)
		})
	}
}

func TestGetProgressFromMessage(t *testing.T) {
	tests := []struct {
		name           string
		message        string
		expectProgress uploader.Progress
	}{
		{
			name:           "error to unmarshall progress",
			message:        "fake-message",
			expectProgress: uploader.Progress{},
		},
		{
			name:    "succeed to unmarshall progress",
			message: "{\"totalBytes\":1000,\"doneBytes\":200}",
			expectProgress: uploader.Progress{
				TotalBytes: 1000,
				BytesDone:  200,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			progress := getProgressFromMessage(test.message, velerotest.NewLogger())
			assert.Equal(t, test.expectProgress, *progress)
		})
	}
}

type redirectFake struct {
	logFile       *os.File
	createTempErr error
	getPodLogErr  error
	logMessage    string
}

func (rf *redirectFake) fakeCreateTempFile(_ string, _ string) (*os.File, error) {
	if rf.createTempErr != nil {
		return nil, rf.createTempErr
	}

	return rf.logFile, nil
}

func (rf *redirectFake) fakeCollectPodLogs(_ context.Context, _ corev1client.CoreV1Interface, _ string, _ string, _ string, output io.Writer) error {
	if rf.getPodLogErr != nil {
		return rf.getPodLogErr
	}

	_, err := output.Write([]byte(rf.logMessage))

	return err
}

func TestRedirectDataMoverLogs(t *testing.T) {
	logFileName := path.Join(os.TempDir(), "test-logger-file.log")

	var buffer string

	tests := []struct {
		name          string
		thisPod       string
		logMessage    string
		logger        logrus.FieldLogger
		createTempErr error
		collectLogErr error
		expectErr     string
	}{
		{
			name:          "error to create temp file",
			thisPod:       "fake-pod",
			createTempErr: errors.New("fake-create-temp-error"),
			logger:        velerotest.NewLogger(),
			expectErr:     "error to create temp file for data mover pod log: fake-create-temp-error",
		},
		{
			name:          "error to collect pod log",
			thisPod:       "fake-pod",
			collectLogErr: errors.New("fake-collect-log-error"),
			logger:        velerotest.NewLogger(),
			expectErr:     fmt.Sprintf("error to collect logs to %s for data mover pod fake-pod: fake-collect-log-error", logFileName),
		},
		{
			name:       "succeed",
			thisPod:    "fake-pod",
			logMessage: "fake-log-message-01\nfake-log-message-02\nfake-log-message-03\n",
			logger:     velerotest.NewSingleLoggerWithHooks(&buffer, logging.DefaultHooks(true)),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buffer = ""

			logFile, err := os.Create(logFileName)
			require.NoError(t, err)

			rf := redirectFake{
				logFile:       logFile,
				createTempErr: test.createTempErr,
				getPodLogErr:  test.collectLogErr,
				logMessage:    test.logMessage,
			}

			funcCreateTemp = rf.fakeCreateTempFile
			funcCollectPodLogs = rf.fakeCollectPodLogs

			fakeKubeClient := kubeclientfake.NewSimpleClientset()

			err = redirectDataMoverLogs(context.Background(), fakeKubeClient, "", test.thisPod, "", test.logger)
			if test.expectErr != "" {
				assert.EqualError(t, err, test.expectErr)
			} else {
				assert.NoError(t, err)

				assert.Contains(t, buffer, test.logMessage)
			}
		})
	}
}
