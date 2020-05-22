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
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	veleroapiv1 "github.com/vmware-tanzu/velero/api/v1"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"

	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// +kubebuilder:scaffold:imports
)

func newFakeClient(g *WithT, initObjs ...runtime.Object) client.Client {
	g.Expect(velerov1api.AddToScheme(scheme.Scheme)).To(Succeed())
	g.Expect(veleroapiv1.AddToScheme(scheme.Scheme)).To(Succeed())
	return fake.NewFakeClientWithScheme(scheme.Scheme, initObjs...)
}
