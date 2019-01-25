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

package output

import (
	"fmt"
	"io"

	"k8s.io/kubernetes/pkg/printers"

	v1 "github.com/heptio/velero/pkg/apis/velero/v1"
)

var (
	resticRepoColumns = []string{"NAME", "STATUS", "LAST MAINTENANCE"}
)

func printResticRepoList(list *v1.ResticRepositoryList, w io.Writer, options printers.PrintOptions) error {
	for i := range list.Items {
		if err := printResticRepo(&list.Items[i], w, options); err != nil {
			return err
		}
	}
	return nil
}

func printResticRepo(repo *v1.ResticRepository, w io.Writer, options printers.PrintOptions) error {
	name := printers.FormatResourceName(options.Kind, repo.Name, options.WithKind)

	if options.WithNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", repo.Namespace); err != nil {
			return err
		}
	}

	status := repo.Status.Phase
	if status == "" {
		status = v1.ResticRepositoryPhaseNew
	}

	lastMaintenance := repo.Status.LastMaintenanceTime.String()
	if repo.Status.LastMaintenanceTime.IsZero() {
		lastMaintenance = "<never>"
	}

	if _, err := fmt.Fprintf(
		w,
		"%s\t%s\t%s",
		name,
		status,
		lastMaintenance,
	); err != nil {
		return err
	}

	if _, err := fmt.Fprint(w, printers.AppendLabels(repo.Labels, options.ColumnLabels)); err != nil {
		return err
	}

	_, err := fmt.Fprint(w, printers.AppendAllLabels(options.ShowLabels, repo.Labels))
	return err
}
