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

import plugin "github.com/hashicorp/go-plugin"

// Handshake returns the configuration information that allows go-plugin clients and servers to perform a handshake.
func Handshake() plugin.HandshakeConfig {
	return plugin.HandshakeConfig{
		// The ProtocolVersion is the version that must match between Velero framework
		// and Velero client plugins. This should be bumped whenever a change happens in
		// one or the other that makes it so that they can't safely communicate.
		ProtocolVersion: 2,

		MagicCookieKey:   "VELERO_PLUGIN",
		MagicCookieValue: "hello",
	}
}
