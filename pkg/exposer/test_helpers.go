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

package exposer

import (
	clientTesting "k8s.io/client-go/testing"
)

// reactor is a helper struct for testing that allows us to customize the reaction to a specific
// verb and resource.
type reactor struct {
	verb        string
	resource    string
	reactorFunc clientTesting.ReactionFunc
}
