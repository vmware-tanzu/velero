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

package client

import (
	"reflect"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/vmware-tanzu/velero/pkg/client/mocks"
)

func TestGetRetriableWithDynamicClient(t *testing.T) {
	type args struct {
		name       string
		getOptions metav1.GetOptions
		retriable  func(error) bool
		errToRetry error
	}
	tests := []struct {
		name    string
		args    args
		want    *unstructured.Unstructured
		wantErr bool
	}{
		{
			name: "retries on not found",
			args: args{
				name:       "foo",
				getOptions: metav1.GetOptions{},
				retriable:  apierrors.IsNotFound,
				errToRetry: apierrors.NewNotFound(schema.GroupResource{}, ""),
			},
			want:    &unstructured.Unstructured{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dc := mocks.NewDynamic(t)
			tries := 0
			dc.On("Get", tt.args.name, tt.args.getOptions).Return(func(name string, getOptions metav1.GetOptions) (*unstructured.Unstructured, error) {
				tries++
				t.Logf("try %d", tries)
				if tries < 3 {
					return nil, tt.args.errToRetry
				}
				return tt.want, nil
			})

			got, err := GetRetriable(GetFuncForDynamicClient(dc, tt.args.getOptions), tt.args.name, tt.args.retriable)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetRetriable(GetFuncForDynamicClient()) error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetRetriable(GetFuncForDynamicClient()) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetRetriableWithCacheLister(t *testing.T) {
	type args struct {
		name       string
		retriable  func(error) bool
		errToRetry error
	}
	tests := []struct {
		name    string
		args    args
		want    runtime.Object
		wantErr bool
	}{
		{
			name: "retries on not found",
			args: args{
				name:       "foo",
				retriable:  apierrors.IsNotFound,
				errToRetry: apierrors.NewNotFound(schema.GroupResource{}, ""),
			},
			want:    &unstructured.Unstructured{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lister := mocks.NewGenericNamespaceLister(t)
			tries := 0
			lister.On("Get", tt.args.name).Return(func(name string) (runtime.Object, error) {
				tries++
				t.Logf("try %d", tries)
				if tries < 3 {
					return nil, tt.args.errToRetry
				}
				return tt.want, nil
			})
			got, err := GetRetriable(GetFuncForCacheLister(lister), tt.args.name, tt.args.retriable)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetRetriable(GetFuncForCacheLister()) error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetRetriable(GetFuncForCacheLister()) = %v, want %v", got, tt.want)
			}
		})
	}
}
