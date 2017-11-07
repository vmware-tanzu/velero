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

package output

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Describe configures a tab writer, passing it to fn. The tab writer's output is returned to the
// caller.
func Describe(fn func(out io.Writer)) string {
	out := new(tabwriter.Writer)
	buf := &bytes.Buffer{}
	out.Init(buf, 0, 8, 2, ' ', 0)

	fn(out)

	out.Flush()
	return buf.String()
}

// DescribeMetadata describes standard object metadata in a consistent manner.
func DescribeMetadata(out io.Writer, metadata metav1.ObjectMeta) {
	fmt.Fprintf(out, "Name:\t%s\n", metadata.Name)
	fmt.Fprintf(out, "Namespace:\t%s\n", metadata.Namespace)
	DescribeMap(out, "Labels", metadata.Labels)
	DescribeMap(out, "Annotations", metadata.Annotations)
}

// DescribeMap describes a map of key-value pairs using name as the heading.
func DescribeMap(out io.Writer, name string, m map[string]string) {
	fmt.Fprintf(out, "%s:\t", name)

	first := true
	prefix := ""
	if len(m) > 0 {
		keys := make([]string, 0, len(m))
		for key := range m {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(out, "%s%s=%s\n", prefix, key, m[key])
			if first {
				first = false
				prefix = "\t"
			}
		}
	} else {
		fmt.Fprint(out, "<none>\n")
	}
}

// DescribeSlice describes a slice of strings using name as the heading. The output is prefixed by
// "preindent" number of tabs.
func DescribeSlice(out io.Writer, preindent int, name string, s []string) {
	pretab := strings.Repeat("\t", preindent)
	fmt.Fprintf(out, "%s%s:\t", pretab, name)

	first := true
	prefix := ""
	if len(s) > 0 {
		for _, x := range s {
			fmt.Fprintf(out, "%s%s\n", prefix, x)
			if first {
				first = false
				prefix = pretab + "\t"
			}
		}
	} else {
		fmt.Fprintf(out, "%s<none>\n", pretab)
	}
}

// BoolPointerString returns the appropriate string based on the bool pointer's value.
func BoolPointerString(b *bool, falseString, trueString, nilString string) string {
	if b == nil {
		return nilString
	}
	if *b {
		return trueString
	}
	return falseString
}
