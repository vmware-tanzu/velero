/*
Copyright 2017 the Heptio Ark contributors.

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

package flag

import "strconv"

type OptionalBool struct {
	Value *bool
}

func NewOptionalBool(defaultValue *bool) OptionalBool {
	return OptionalBool{
		Value: defaultValue,
	}
}

// String returns a string representation of the
// enum flag.
func (f *OptionalBool) String() string {
	switch f.Value {
	case nil:
		return "<nil>"
	default:
		return strconv.FormatBool(*f.Value)
	}
}

func (f *OptionalBool) Set(val string) error {
	if val == "" {
		f.Value = nil
		return nil
	}

	parsed, err := strconv.ParseBool(val)
	if err != nil {
		return err
	}

	f.Value = &parsed

	return nil
}

func (f *OptionalBool) Type() string {
	return "optionalBool"
}
