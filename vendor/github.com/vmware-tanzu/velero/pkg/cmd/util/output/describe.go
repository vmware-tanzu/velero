/*
Copyright 2017 the Velero contributors.

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
	"sort"
	"strings"
	"text/tabwriter"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Describer struct {
	Prefix string
	out    *tabwriter.Writer
	buf    *bytes.Buffer
}

func Describe(fn func(d *Describer)) string {
	d := Describer{
		out: new(tabwriter.Writer),
		buf: new(bytes.Buffer),
	}
	d.out.Init(d.buf, 0, 8, 2, ' ', 0)

	fn(&d)

	d.out.Flush()
	return d.buf.String()
}

func (d *Describer) Printf(msg string, args ...interface{}) {
	fmt.Fprint(d.out, d.Prefix)
	fmt.Fprintf(d.out, msg, args...)
}

func (d *Describer) Println(args ...interface{}) {
	fmt.Fprint(d.out, d.Prefix)
	fmt.Fprintln(d.out, args...)
}

// DescribeMetadata describes standard object metadata in a consistent manner.
func (d *Describer) DescribeMetadata(metadata metav1.ObjectMeta) {
	d.Printf("Name:\t%s\n", metadata.Name)
	d.Printf("Namespace:\t%s\n", metadata.Namespace)
	d.DescribeMap("Labels", metadata.Labels)
	d.DescribeMap("Annotations", metadata.Annotations)
}

// DescribeMap describes a map of key-value pairs using name as the heading.
func (d *Describer) DescribeMap(name string, m map[string]string) {
	d.Printf("%s:\t", name)

	first := true
	prefix := ""
	if len(m) > 0 {
		keys := make([]string, 0, len(m))
		for key := range m {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			d.Printf("%s%s=%s\n", prefix, key, m[key])
			if first {
				first = false
				prefix = "\t"
			}
		}
	} else {
		d.Printf("<none>\n")
	}
}

// DescribeSlice describes a slice of strings using name as the heading. The output is prefixed by
// "preindent" number of tabs.
func (d *Describer) DescribeSlice(preindent int, name string, s []string) {
	pretab := strings.Repeat("\t", preindent)
	d.Printf("%s%s:\t", pretab, name)

	first := true
	prefix := ""
	if len(s) > 0 {
		for _, x := range s {
			d.Printf("%s%s\n", prefix, x)
			if first {
				first = false
				prefix = pretab + "\t"
			}
		}
	} else {
		d.Printf("%s<none>\n", pretab)
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
