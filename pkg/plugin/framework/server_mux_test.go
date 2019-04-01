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
	"strings"
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
		// test prefix for fqdn

		// test general format
		{
			name: "empty",
			args: args{
				name:          "",
				existingNames: []string{"velero.io/aws"},
			},
			want: false,
		},
		{
			name: "name same as separator",
			args: args{
				name:          "/",
				existingNames: []string{"velero.io/aws"},
			},
			want: false,
		},
		{
			name: "two separators",
			args: args{
				name:          "//",
				existingNames: []string{"velero.io/aws"},
			},
			want: false,
		},

		// test combination
		{
			name: "prefix but no name",
			args: args{
				name:          "a/",
				existingNames: []string{"velero.io/aws"},
			},
			want: false,
		},
		{
			name: "name but no prefix",
			args: args{
				name:          "/a",
				existingNames: []string{"velero.io/aws"},
			},
			want: false,
		},
		{
			name: "existing prefix/name",
			args: args{
				name:          "velero.io/aws",
				existingNames: []string{"velero.io/aws"},
			},
			want: false,
		},
		{
			name: "valid prefix/name",
			args: args{
				name:          "example.io/azure",
				existingNames: []string{"velero.io/aws"},
			},
			want: true,
		},
	}

	successCases := []string{
		"simple",
		"now-with-dashes",
		"1-starts-with-num",
		"1234",
		"simple/simple",
		"now-with-dashes/simple",
		"now-with-dashes/now-with-dashes",
		"now.with.dots/simple",
		"now-with.dashes-and.dots/simple",
		"1-num.2-num/3-num",
		"1234/5678",
		"1.2.3.4/5678",
		"Uppercase_Is_OK_123",
		"example.com/Uppercase_Is_OK_123",
		"requests.storage-foo",
		strings.Repeat("a", 63),
		strings.Repeat("a", 253) + "/" + strings.Repeat("b", 63),
	}

	for _, tt := range successCases {
		t.Run(tt.name, func(t *testing.T) {
			if errs := validPluginName(successCases[i]); len(errs) != 0 {
				t.Errorf("case[%d]: %q: expected success: %v", i, successCases[i], errs)
			}

			if got := validPluginName(tt.args.name, tt.args.existingNames); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}

	errorCases := []string{
		"nospecialchars%^=@",
		"cantendwithadash-",
		"-cantstartwithadash-",
		"only/one/slash",
		"Example.com/abc",
		"example_com/abc",
		"example.com/",
		"/simple",
		strings.Repeat("a", 64),
		strings.Repeat("a", 254) + "/abc",
	}
}

// func Test_fqdnValidation(t *testing.T) {
// 	tests := []struct {
// 		tld  string
// 		want bool
// 	}{
// 		{"test.example.com", true},
// 		{"example.com", true},
// 		{"example24.com", true},
// 		{"test.example24.com", true},
// 		{"test24.example24.com", true},
// 		{"test.example.com.", true},
// 		{"example.com.", true},
// 		{"example24.com.", true},
// 		{"test.example24.com.", true},
// 		{"test24.example24.com.", true},
// 		{"test24.example24.com..", false},
// 		{"example", false},
// 		{"192.168.0.1", false},
// 		{"email@example.com", false},
// 		{"2001:cdba:0000:0000:0000:0000:3257:9652", false},
// 		{"2001:cdba:0:0:0:0:3257:9652", false},
// 		{"2001:cdba::3257:9652", false},
// 		{"", false},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.tld, func(t *testing.T) {
// 			if got := fqdnValidation(tt.tld); got != tt.want {

// 				t.Errorf("got %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }
