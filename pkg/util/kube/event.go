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
package kube

import (
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
)

type EventRecorder interface {
	Event(object runtime.Object, warning bool, reason string, message string, a ...any)
	Shutdown()
}

type eventRecorder struct {
	broadcaster record.EventBroadcaster
	recorder    record.EventRecorder
}

func NewEventRecorder(kubeClient kubernetes.Interface, scheme *runtime.Scheme, eventSource string, eventNode string) EventRecorder {
	res := eventRecorder{}

	res.broadcaster = record.NewBroadcasterWithCorrelatorOptions(record.CorrelatorOptions{
		MaxEvents: 1,
		MessageFunc: func(event *v1.Event) string {
			return event.Message
		},
	})

	res.broadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	res.recorder = res.broadcaster.NewRecorder(scheme, v1.EventSource{
		Component: eventSource,
		Host:      eventNode,
	})

	return &res
}

func (er *eventRecorder) Event(object runtime.Object, warning bool, reason string, message string, a ...any) {
	eventType := v1.EventTypeNormal
	if warning {
		eventType = v1.EventTypeWarning
	}

	if len(a) > 0 {
		er.recorder.Eventf(object, eventType, reason, message, a...)
	} else {
		er.recorder.Event(object, eventType, reason, message)
	}
}

func (er *eventRecorder) Shutdown() {
	// StartEventWatcher doesn't wait for writing all buffered events to API server when Shutdown is called, so have to hardcode a sleep time
	time.Sleep(2 * time.Second)
	er.broadcaster.Shutdown()
}
