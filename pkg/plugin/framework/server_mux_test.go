/*
Copyright 2019 the Velero contributors.

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

package framework

import (
	"testing"
)

func Test_validPluginName(t *testing.T) {
	type args struct {
		name          string
		existingNames []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "empty",
			args: args{
				name:          "",
				existingNames: []string{"velerio.io/aws"},
			},
			want: false,
		},
		{
			name: "name same as separator",
			args: args{
				name:          "/",
				existingNames: []string{"velerio.io/aws"},
			},
			want: false,
		},
		{
			name: "two separators",
			args: args{
				name:          "//",
				existingNames: []string{"velerio.io/aws"},
			},
			want: false,
		},
		{
			name: "namespace but no name",
			args: args{
				name:          "a/",
				existingNames: []string{"velerio.io/aws"},
			},
			want: false,
		},
		{
			name: "name but no namespace",
			args: args{
				name:          "/a",
				existingNames: []string{"velerio.io/aws"},
			},
			want: false,
		},
		{
			name: "gonna fail",
			args: args{
				name:          "velerio.io/aws",
				existingNames: []string{"velerio.io/aws"},
			},
			want: false,
		},
		{
			name: "gonna fail",
			args: args{
				name:          "example/azure",
				existingNames: []string{"velerio.io/aws"},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validPluginName(tt.args.name, tt.args.existingNames); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
