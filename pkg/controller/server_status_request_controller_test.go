/*
Copyright 2020 the Velero contributors.

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

package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/reynencourt/velero/internal/velero"
	velerov1api "github.com/reynencourt/velero/pkg/apis/velero/v1"
	"github.com/reynencourt/velero/pkg/builder"
	"github.com/reynencourt/velero/pkg/buildinfo"
	"github.com/reynencourt/velero/pkg/plugin/framework"
	velerotest "github.com/reynencourt/velero/pkg/test"
)

func statusRequestBuilder(resourceVersion string) *builder.ServerStatusRequestBuilder {
	return builder.ForServerStatusRequest(velerov1api.DefaultNamespace, "sr-1", resourceVersion)
}

var _ = Describe("Server Status Request Reconciler", func() {
	BeforeEach(func() {})
	AfterEach(func() {})

	It("Should successfully patch a server status request object status phase", func() {
		// now will be used to set the fake clock's time; capture
		// it here so it can be referenced in the test case defs.
		now, err := time.Parse(time.RFC1123, time.RFC1123)
		Expect(err).To(BeNil())
		now = now.Local()

		tests := []struct {
			req             *velerov1api.ServerStatusRequest
			reqPluginLister *fakePluginLister
			expected        *velerov1api.ServerStatusRequest
			expectedRequeue ctrl.Result
			expectedErrMsg  string
		}{
			{
				// server status request with phase=empty will be processed
				req: statusRequestBuilder("1").
					ServerVersion(buildinfo.Version).
					ProcessedTimestamp(now).
					Plugins([]velerov1api.PluginInfo{
						{
							Name: "custom.io/myown",
							Kind: "VolumeSnapshotter",
						},
					}).
					Result(),
				reqPluginLister: &fakePluginLister{
					plugins: []framework.PluginIdentifier{
						{
							Name: "custom.io/myown",
							Kind: "VolumeSnapshotter",
						},
					},
				},
				expected: statusRequestBuilder("1").
					ServerVersion(buildinfo.Version).
					Phase(velerov1api.ServerStatusRequestPhaseProcessed).
					ProcessedTimestamp(now).
					Plugins([]velerov1api.PluginInfo{
						{
							Name: "custom.io/myown",
							Kind: "VolumeSnapshotter",
						},
					}).
					Result(),
				expectedRequeue: ctrl.Result{Requeue: false, RequeueAfter: statusRequestResyncPeriod},
			},
			{
				// server status request with phase=new will be processed
				req: statusRequestBuilder("1").
					ServerVersion(buildinfo.Version).
					Phase(velerov1api.ServerStatusRequestPhaseNew).
					ProcessedTimestamp(now).
					Plugins([]velerov1api.PluginInfo{
						{
							Name: "custom.io/myown",
							Kind: "VolumeSnapshotter",
						},
					}).
					Result(),
				reqPluginLister: &fakePluginLister{
					plugins: []framework.PluginIdentifier{
						{
							Name: "custom.io/myown",
							Kind: "VolumeSnapshotter",
						},
					},
				},
				expected: statusRequestBuilder("1").
					ServerVersion(buildinfo.Version).
					Phase(velerov1api.ServerStatusRequestPhaseProcessed).
					ProcessedTimestamp(now).
					Plugins([]velerov1api.PluginInfo{
						{
							Name: "custom.io/myown",
							Kind: "VolumeSnapshotter",
						},
					}).
					Result(),
				expectedRequeue: ctrl.Result{Requeue: false, RequeueAfter: statusRequestResyncPeriod},
			},
			{
				// server status request with phase=Processed does not get deleted if not expired
				req: statusRequestBuilder("1").
					ServerVersion(buildinfo.Version).
					Phase(velerov1api.ServerStatusRequestPhaseProcessed).
					ProcessedTimestamp(now). // not yet expired
					Plugins([]velerov1api.PluginInfo{
						{
							Name: "custom.io/myotherown",
							Kind: "VolumeSnapshotter",
						},
					}).
					Result(),
				reqPluginLister: &fakePluginLister{
					plugins: []framework.PluginIdentifier{
						{
							Name: "custom.io/myotherown",
							Kind: "VolumeSnapshotter",
						},
					},
				},
				expected: statusRequestBuilder("1").
					ServerVersion(buildinfo.Version).
					Phase(velerov1api.ServerStatusRequestPhaseProcessed).
					ProcessedTimestamp(now).
					Plugins([]velerov1api.PluginInfo{
						{
							Name: "custom.io/myown",
							Kind: "VolumeSnapshotter",
						},
					}).
					Result(),
				expectedRequeue: ctrl.Result{Requeue: false, RequeueAfter: statusRequestResyncPeriod},
			},
			{
				// server status request with phase=Processed gets deleted if expire
				req: statusRequestBuilder("1").
					ServerVersion(buildinfo.Version).
					Phase(velerov1api.ServerStatusRequestPhaseProcessed).
					ProcessedTimestamp(now.Add(-61 * time.Second)). // expired
					Plugins([]velerov1api.PluginInfo{
						{
							Name: "custom.io/myotherown",
							Kind: "VolumeSnapshotter",
						},
					}).
					Result(),
				reqPluginLister: &fakePluginLister{
					plugins: []framework.PluginIdentifier{
						{
							Name: "custom.io/myotherown",
							Kind: "VolumeSnapshotter",
						},
					},
				},
				expected:        nil,
				expectedRequeue: ctrl.Result{Requeue: false, RequeueAfter: statusRequestResyncPeriod},
			},
			{
				// server status request with invalid phase returns an error and does not requeue
				req: statusRequestBuilder("1").
					ServerVersion(buildinfo.Version).
					Phase("an-invalid-phase").
					ProcessedTimestamp(now).
					Plugins([]velerov1api.PluginInfo{
						{
							Name: "custom.io/myown",
							Kind: "VolumeSnapshotter",
						},
					}).
					Result(),
				reqPluginLister: &fakePluginLister{
					plugins: []framework.PluginIdentifier{
						{
							Name: "custom.io/myown",
							Kind: "VolumeSnapshotter",
						},
					},
				},
				expectedErrMsg:  "unexpected ServerStatusRequest phase",
				expectedRequeue: ctrl.Result{Requeue: false, RequeueAfter: 0},
			},
		}

		for _, test := range tests {
			// Setup reconciler
			Expect(velerov1api.AddToScheme(scheme.Scheme)).To(Succeed())
			serverStatusInfo := velero.ServerStatus{
				PluginRegistry: test.reqPluginLister,
				Clock:          clock.NewFakeClock(now),
			}
			r := ServerStatusRequestReconciler{
				Client:       fake.NewFakeClientWithScheme(scheme.Scheme, test.req),
				ServerStatus: serverStatusInfo,
				Ctx:          context.Background(),
				Log:          velerotest.NewLogger(),
			}

			actualResult, err := r.Reconcile(ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: velerov1api.DefaultNamespace,
					Name:      test.req.Name,
				},
			})

			Expect(actualResult).To(BeEquivalentTo(test.expectedRequeue))
			if test.expectedErrMsg == "" {
				Expect(err).To(BeNil())
			} else {
				Expect(err.Error()).To(BeEquivalentTo(test.expectedErrMsg))
				return
			}

			instance := &velerov1api.ServerStatusRequest{}
			err = r.Client.Get(ctx, kbclient.ObjectKey{Name: test.req.Name, Namespace: test.req.Namespace}, instance)

			// Assertions
			if test.expected == nil {
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			} else {
				Expect(err).To(BeNil())
				Eventually(instance.Status.Phase == test.expected.Status.Phase, timeout).Should(BeTrue())
			}
		}
	})
})

type fakePluginLister struct {
	plugins []framework.PluginIdentifier
}

func (l *fakePluginLister) List(kind framework.PluginKind) []framework.PluginIdentifier {
	var plugins []framework.PluginIdentifier
	for _, plugin := range l.plugins {
		if plugin.Kind == kind {
			plugins = append(plugins, plugin)
		}
	}

	return plugins
}
