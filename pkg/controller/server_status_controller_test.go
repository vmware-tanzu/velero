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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/vmware-tanzu/velero/internal/velero"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/buildinfo"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
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
			expectedPhase   velerov1api.ServerStatusRequestPhase
		}{
			{
				req: statusRequestBuilder("1").
					ServerVersion(buildinfo.Version).
					// Phase(velerov1api.ServerStatusRequestPhaseNew).
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
				expectedPhase: velerov1api.ServerStatusRequestPhaseProcessed,
			},
			// {
			// 	req: statusRequestBuilder("1").
			// 		ServerVersion(buildinfo.Version).
			// 		Phase(velerov1api.ServerStatusRequestPhaseProcessed).
			// 		ProcessedTimestamp(now).
			// 		Plugins([]velerov1api.PluginInfo{
			// 			{
			// 				Name: "custom.io/myown",
			// 				Kind: "VolumeSnapshotter",
			// 			},
			// 		}).
			// 		Result(),
			// 	reqPluginLister: &fakePluginLister{
			// 		plugins: []framework.PluginIdentifier{
			// 			{
			// 				Name: "custom.io/myown",
			// 				Kind: "VolumeSnapshotter",
			// 			},
			// 		},
			// 	},
			// 	expectedPhase: velerov1api.ServerStatusRequestPhaseNew,
			// },
		}

		var r ServerStatusRequestReconciler

		// serverRequests := new(velerov1api.ServerStatusRequestList)
		for _, test := range tests {
			// serverRequest := test.req
			// serverRequests.Items = append(serverRequests.Items, *serverRequest)

			// Setup reconciler
			Expect(velerov1api.AddToScheme(scheme.Scheme)).To(Succeed())

			k8sClient := fake.NewFakeClientWithScheme(scheme.Scheme, test.req)

			serverStatusInfo := velero.ServerStatus{
				Client:         k8sClient,
				PluginRegistry: test.reqPluginLister,
				Clock:          clock.NewFakeClock(now),
			}

			r = ServerStatusRequestReconciler{
				// Client:       fake.NewFakeClientWithScheme(scheme.Scheme, serverRequests),
				Client:       k8sClient,
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

			Expect(actualResult).To(BeEquivalentTo(ctrl.Result{RequeueAfter: statusRequestResyncPeriod}))
			Expect(err).To(BeNil())

			srs := &velerov1api.ServerStatusRequest{}
			// Wait for reconciliation to happen.
			Eventually(func() bool {
				if err := k8sClient.Get(ctx,
					client.ObjectKey{Name: test.req.Name, Namespace: test.req.Namespace}, srs); err != nil {
					return false
				}
				return srs.Status.Phase == velerov1api.ServerStatusRequestPhaseProcessed
			}, timeout).Should(BeTrue())

			// time.Sleep(3 * time.Minute)
			// r.Reconcile(ctrl.Request{
			// 	NamespacedName: types.NamespacedName{
			// 		Namespace: velerov1api.DefaultNamespace,
			// 		Name:      test.req.Name,
			// 	},
			// })

		}

		requests := new(velerov1api.ServerStatusRequestList)
		err = r.Client.List(context.Background(), requests, &kbclient.ListOptions{
			Namespace: velerov1api.DefaultNamespace,
		})
		Expect(err).To(BeNil())

		for _, test := range tests {
			// Assertion

			// _, err := r.Reconcile(ctrl.Request{
			// 	NamespacedName: types.NamespacedName{
			// 		Namespace: velerov1api.DefaultNamespace,
			// 		Name:      test.req.Name,
			// 	},
			// })
			// Expect(err).To(BeNil())

			key := client.ObjectKey{Name: test.req.Name, Namespace: velerov1api.DefaultNamespace}
			instance := &velerov1api.ServerStatusRequest{}
			err = r.Client.Get(ctx, key, instance)
			Expect(err).To(BeNil())
			// Expect(err).ToNot(BeNil())
			Expect(instance.Status.Phase).To(BeIdenticalTo(test.expectedPhase))
		}

		// Assertions
		// for i, serverRequest := range serverRequests.Items {
		// 	key := client.ObjectKey{Name: serverRequest.Name, Namespace: serverRequest.Namespace}
		// 	instance := &velerov1api.ServerStatusRequest{}
		// 	err := r.Client.Get(ctx, key, instance)
		// 	Expect(err).To(BeNil())
		// 	Expect(instance.Status.Phase).To(BeIdenticalTo(tests[i].expectedPhase))
		// }

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
