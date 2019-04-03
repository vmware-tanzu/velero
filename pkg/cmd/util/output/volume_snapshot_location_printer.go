/*
Copyright 2018 the Velero contributors.

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
	"fmt"
	"io"

	"k8s.io/kubernetes/pkg/printers"

	v1 "github.com/heptio/velero/pkg/apis/velero/v1"
)

var (
	volumeSnapshotLocationColumns = []string{"NAME", "PROVIDER"}
)

func printVolumeSnapshotLocationList(list *v1.VolumeSnapshotLocationList, w io.Writer, options printers.PrintOptions) error {
	for i := range list.Items {
		if err := printVolumeSnapshotLocation(&list.Items[i], w, options); err != nil {
			return err
		}
	}
	return nil
}

func printVolumeSnapshotLocation(location *v1.VolumeSnapshotLocation, w io.Writer, options printers.PrintOptions) error {
	name := printers.FormatResourceName(options.Kind, location.Name, options.WithKind)

	if options.WithNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", location.Namespace); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(
		w,
		"%s\t%s",
		name,
		location.Spec.Provider,
	); err != nil {
		return err
	}

	if _, err := fmt.Fprint(w, printers.AppendLabels(location.Labels, options.ColumnLabels)); err != nil {
		return err
	}

	_, err := fmt.Fprint(w, printers.AppendAllLabels(options.ShowLabels, location.Labels))
	return err
}
