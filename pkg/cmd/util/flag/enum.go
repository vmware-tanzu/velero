/*
Copyright 2017 Heptio Inc.

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

import (
	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/util/sets"
)

// Enum is a Cobra-compatible wrapper for defining
// a string flag that can be one of a specified set
// of values.
type Enum struct {
	allowedValues sets.String
	value         string
}

// NewEnum returns a new enum flag with the specified list
// of allowed values. The first value specified is used
// as the default.
func NewEnum(allowedValues ...string) Enum {
	return Enum{
		allowedValues: sets.NewString(allowedValues...),
		value:         allowedValues[0],
	}
}

// String returns a string representation of the
// enum flag.
func (e *Enum) String() string {
	return e.value
}

// Set assigns the provided string to the enum
// receiver. It returns an error if the string
// is not an allowed value.
func (e *Enum) Set(s string) error {
	if !e.allowedValues.Has(s) {
		return errors.Errorf("invalid value: %q", s)
	}

	e.value = s
	return nil
}

// Type returns a string representation of the
// Enum type.
func (e *Enum) Type() string {
	return "enum"
}
