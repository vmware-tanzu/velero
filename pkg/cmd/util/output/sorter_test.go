/*
Copyright the Velero contributors.

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

package output

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func createPodSpecResource(t *testing.T, memReq, memLimit, cpuReq, cpuLimit string) corev1.PodSpec {
	t.Helper()
	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{},
					Limits:   corev1.ResourceList{},
				},
			},
		},
	}

	req := podSpec.Containers[0].Resources.Requests
	if memReq != "" {
		memReq, err := resource.ParseQuantity(memReq)
		if err != nil {
			t.Errorf("memory request string is not a valid quantity")
		}
		req["memory"] = memReq
	}
	if cpuReq != "" {
		cpuReq, err := resource.ParseQuantity(cpuReq)
		if err != nil {
			t.Errorf("cpu request string is not a valid quantity")
		}
		req["cpu"] = cpuReq
	}
	limit := podSpec.Containers[0].Resources.Limits
	if memLimit != "" {
		memLimit, err := resource.ParseQuantity(memLimit)
		if err != nil {
			t.Errorf("memory limit string is not a valid quantity")
		}
		limit["memory"] = memLimit
	}
	if cpuLimit != "" {
		cpuLimit, err := resource.ParseQuantity(cpuLimit)
		if err != nil {
			t.Errorf("cpu limit string is not a valid quantity")
		}
		limit["cpu"] = cpuLimit
	}

	return podSpec
}

func TestRuntimeSortLess(t *testing.T) {
	var testobj runtime.Object

	testobj = &corev1.PodList{
		Items: []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "b",
				},
				Spec: createPodSpecResource(t, "0.5", "", "1Gi", ""),
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "c",
				},
				Spec: createPodSpecResource(t, "2", "", "1Ti", ""),
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "a",
				},
				Spec: createPodSpecResource(t, "10m", "", "1Ki", ""),
			},
		},
	}

	testobjs, err := meta.ExtractList(testobj)
	if err != nil {
		t.Fatalf("ExtractList testobj got unexpected error: %v", err)
	}

	testfieldName := "{.metadata.name}"
	testruntimeSortName := NewRuntimeSort(testfieldName, testobjs)

	testfieldCPU := "{.spec.containers[].resources.requests.cpu}"
	testruntimeSortCPU := NewRuntimeSort(testfieldCPU, testobjs)

	testfieldMemory := "{.spec.containers[].resources.requests.memory}"
	testruntimeSortMemory := NewRuntimeSort(testfieldMemory, testobjs)

	tests := []struct {
		name         string
		runtimeSort  *RuntimeSort
		i            int
		j            int
		expectResult bool
		expectErr    bool
	}{
		{
			name:         "test name b c less true",
			runtimeSort:  testruntimeSortName,
			i:            0,
			j:            1,
			expectResult: true,
		},
		{
			name:         "test name c a less false",
			runtimeSort:  testruntimeSortName,
			i:            1,
			j:            2,
			expectResult: false,
		},
		{
			name:         "test name b a less false",
			runtimeSort:  testruntimeSortName,
			i:            0,
			j:            2,
			expectResult: false,
		},
		{
			name:         "test cpu 0.5 2 less true",
			runtimeSort:  testruntimeSortCPU,
			i:            0,
			j:            1,
			expectResult: true,
		},
		{
			name:         "test cpu 2 10mi less false",
			runtimeSort:  testruntimeSortCPU,
			i:            1,
			j:            2,
			expectResult: false,
		},
		{
			name:         "test cpu 0.5 10mi less false",
			runtimeSort:  testruntimeSortCPU,
			i:            0,
			j:            2,
			expectResult: false,
		},
		{
			name:         "test memory 1Gi 1Ti less true",
			runtimeSort:  testruntimeSortMemory,
			i:            0,
			j:            1,
			expectResult: true,
		},
		{
			name:         "test memory 1Ti 1Ki less false",
			runtimeSort:  testruntimeSortMemory,
			i:            1,
			j:            2,
			expectResult: false,
		},
		{
			name:         "test memory 1Gi 1Ki less false",
			runtimeSort:  testruntimeSortMemory,
			i:            0,
			j:            2,
			expectResult: false,
		},
	}

	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := test.runtimeSort.Less(test.i, test.j)
			if result != test.expectResult {
				t.Errorf("case[%d]:%s Expected result: %v, Got result: %v", i, test.name, test.expectResult, result)
			}
		})
	}
}
