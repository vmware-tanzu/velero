/*
Copyright 2018, 2019 the Velero contributors.

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
package builder

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetName(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected string
	}{
		{
			name:     "image name with registry hostname and tag",
			image:    "gcr.io/my-repo/my-image:latest",
			expected: "my-repo-my-image",
		},
		{
			name:     "image name with registry hostname, without tag",
			image:    "gcr.io/my-repo/my-image",
			expected: "my-repo-my-image",
		},
		{
			name:     "image name without registry hostname, with tag",
			image:    "my-repo/my-image:latest",
			expected: "my-repo-my-image",
		},
		{
			name:     "image name without registry hostname, without tag",
			image:    "my-repo/my-image",
			expected: "my-repo-my-image",
		},
		{
			name:     "image name with registry hostname and port, and tag",
			image:    "mycustomregistry.io:8080/my-repo/my-image:latest",
			expected: "my-repo-my-image",
		},
		{
			name:     "image name with no / in it",
			image:    "my-image",
			expected: "my-image",
		},
		{
			name:     "image name starting with / in it",
			image:    "/my-image",
			expected: "my-image",
		},
		{
			name:     "image name with repo starting with a / as first char",
			image:    "/my-repo/my-image",
			expected: "my-repo-my-image",
		},
		{
			name:     "image name with registry hostname, etoomany slashes, without tag",
			image:    "gcr.io/my-repo/mystery/another/my-image",
			expected: "my-repo-mystery-another-my-image",
		},
		{
			name:     "image name with registry hostname starting with a / will include the registry name ¯\\_(ツ)_/¯",
			image:    "/gcr.io/my-repo/mystery/another/my-image",
			expected: "gcr-io-my-repo-mystery-another-my-image",
		},
		{
			name:     "image repository names containing _ ",
			image:    "projects.registry.vmware.com/tanzu_migrator/route-2-httpproxy:myTag",
			expected: "tanzu-migrator-route-2-httpproxy",
		},
		{
			name:     "image repository names containing . ",
			image:    "projects.registry.vmware.com/tanzu.migrator/route-2-httpproxy:myTag",
			expected: "tanzu-migrator-route-2-httpproxy",
		},
		{
			name:     "pull by digest",
			image:    "quay.io/vmware-tanzu/velero@sha256:a75f9e8c3ced3943515f249597be389f8233e1258d289b11184796edceaa7dab",
			expected: "vmware-tanzu-velero",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, getName(test.image))
		})
	}
}

func TestEnsureValidNameLength(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		validate func(t *testing.T, result string)
	}{
		{
			name:   "short name unchanged",
			input:  "my-short-name",
			maxLen: 63,
			validate: func(t *testing.T, result string) {
				t.Helper()
				assert.Equal(t, "my-short-name", result)
				assert.LessOrEqual(t, len(result), 63)
			},
		},
		{
			name:   "name exactly at limit unchanged",
			input:  "this-is-exactly-sixty-three-characters-long-name-padding-xxxxxx",
			maxLen: 63,
			validate: func(t *testing.T, result string) {
				t.Helper()
				assert.Equal(t, "this-is-exactly-sixty-three-characters-long-name-padding-xxxxxx", result)
				assert.Len(t, result, 63)
			},
		},
		{
			name:   "long name gets truncated with hash",
			input:  "this-is-a-very-long-container-name-that-exceeds-sixty-three-characters-limit",
			maxLen: 63,
			validate: func(t *testing.T, result string) {
				t.Helper()
				assert.LessOrEqual(t, len(result), 63)
				assert.NotEqual(t, "this-is-a-very-long-container-name-that-exceeds-sixty-three-characters-limit", result)
				// Should end with hash suffix
				assert.Contains(t, result, "-")
				// Hash should be consistent
				result2 := ensureValidNameLength("this-is-a-very-long-container-name-that-exceeds-sixty-three-characters-limit")
				assert.Equal(t, result, result2)
			},
		},
		{
			name:   "real-world plugin image path with deeply nested repository",
			input:  "redhat-user-workloads-ocp-art-tenant-oadp-hypershift-oadp-plugin-main",
			maxLen: 63,
			validate: func(t *testing.T, result string) {
				t.Helper()
				assert.LessOrEqual(t, len(result), 63)
				assert.Len(t, result, 63)
				// Verify it's deterministic
				result2 := ensureValidNameLength("redhat-user-workloads-ocp-art-tenant-oadp-hypershift-oadp-plugin-main")
				assert.Equal(t, result, result2)
			},
		},
		{
			name:   "different long names produce different results",
			input:  "different-very-long-container-name-that-also-exceeds-the-sixty-three-character-limit",
			maxLen: 63,
			validate: func(t *testing.T, result string) {
				t.Helper()
				assert.LessOrEqual(t, len(result), 63)
				other := ensureValidNameLength("another-very-long-container-name-that-also-exceeds-the-sixty-three-character-limit")
				assert.NotEqual(t, result, other)
			},
		},
		{
			name:   "empty string",
			input:  "",
			maxLen: 63,
			validate: func(t *testing.T, result string) {
				t.Helper()
				assert.Empty(t, result)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := ensureValidNameLength(test.input)
			test.validate(t, result)
		})
	}
}

func TestGetNameWithLongPaths(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		validate func(t *testing.T, result string)
	}{
		{
			name:  "plugin with deeply nested repository path exceeding 63 characters",
			image: "arohcpsvcdev.azurecr.io/redhat-user-workloads/ocp-art-tenant/oadp-hypershift-oadp-plugin-main@sha256:adb840bf3890b4904a8cdda1a74c82cf8d96c52eba9944ac10e795335d6fd450",
			validate: func(t *testing.T, result string) {
				t.Helper()
				// Should not exceed DNS-1123 label limit of 63 characters
				assert.LessOrEqual(t, len(result), 63, "Container name must satisfy DNS-1123 label constraints (max 63 chars)")
				// Should be exactly 63 characters (truncated with hash)
				assert.Len(t, result, 63)
				// Should be deterministic
				result2 := getName("arohcpsvcdev.azurecr.io/redhat-user-workloads/ocp-art-tenant/oadp-hypershift-oadp-plugin-main@sha256:adb840bf3890b4904a8cdda1a74c82cf8d96c52eba9944ac10e795335d6fd450")
				assert.Equal(t, result, result2)
			},
		},
		{
			name:  "plugin with normal path length (should remain unchanged)",
			image: "arohcpsvcdev.azurecr.io/konveyor/velero-plugin-for-microsoft-azure@sha256:b2db5f09da514e817a74c992dcca5f90b77c2ab0b2797eba947d224271d6070e",
			validate: func(t *testing.T, result string) {
				t.Helper()
				assert.Equal(t, "konveyor-velero-plugin-for-microsoft-azure", result)
				assert.LessOrEqual(t, len(result), 63)
			},
		},
		{
			name:  "very long nested path",
			image: "registry.example.com/org/team/project/subproject/component/service/application-name-with-many-words:v1.2.3",
			validate: func(t *testing.T, result string) {
				t.Helper()
				assert.LessOrEqual(t, len(result), 63)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := getName(test.image)
			test.validate(t, result)
		})
	}
}

func TestHashString(t *testing.T) {
	// Test that hashString produces consistent results
	input := "test-string"
	hash1 := hashString(input)
	hash2 := hashString(input)
	assert.Equal(t, hash1, hash2, "Hash should be deterministic")

	// Test that different inputs produce different hashes
	hash3 := hashString("different-string")
	assert.NotEqual(t, hash1, hash3, "Different inputs should produce different hashes")

	// Test hash length (SHA256 produces 64 hex characters)
	assert.Len(t, hash1, 64, "SHA256 hash should be 64 hex characters")
}
