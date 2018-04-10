/*
Copyright 2018 the Heptio Ark contributors.

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

package errcheck

import "errors"

// ErrOrNotFound return an error if err is not nil, or if
// found is false (in which case a new error is created
// with the specified msg), or nil otherwise.
func ErrOrNotFound(found bool, err error, msg string) error {
	switch {
	case err != nil:
		return err
	case !found:
		return errors.New(msg)
	default:
		return nil
	}
}
