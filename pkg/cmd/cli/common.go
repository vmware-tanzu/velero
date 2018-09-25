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

package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// GetConfirmation ensures that the user confirms the action before proceeding.
func GetConfirmation() bool {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("Are you sure you want to continue (Y/N)? ")

		confirmation, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading user input: %v\n", err)
			return false
		}
		confirmation = strings.TrimSpace(confirmation)
		if len(confirmation) != 1 {
			continue
		}

		switch strings.ToLower(confirmation) {
		case "y":
			return true
		case "n":
			return false
		}
	}
}

// Xor returns true if exactly one of the provided values is true,
// or false otherwise.
func Xor(val bool, vals ...bool) bool {
	res := val

	for _, v := range vals {
		if res && v {
			return false
		}
		res = res || v
	}
	return res
}
