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

package output

import (
	"fmt"
	"io"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/printers"

	"github.com/heptio/ark/pkg/apis/ark/v1"
)

var (
	scheduleColumns = []string{"NAME", "STATUS", "CREATED", "SCHEDULE", "BACKUP TTL", "LAST BACKUP", "SELECTOR"}
)

func printScheduleList(list *v1.ScheduleList, w io.Writer, options printers.PrintOptions) error {
	for i := range list.Items {
		if err := printSchedule(&list.Items[i], w, options); err != nil {
			return err
		}
	}
	return nil
}

func printSchedule(schedule *v1.Schedule, w io.Writer, options printers.PrintOptions) error {
	name := printers.FormatResourceName(options.Kind, schedule.Name, options.WithKind)

	if options.WithNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", schedule.Namespace); err != nil {
			return err
		}
	}

	status := schedule.Status.Phase
	if status == "" {
		status = v1.SchedulePhaseNew
	}

	_, err := fmt.Fprintf(
		w,
		"%s\t%s\t%s\t%s\t%s\t%s\t%s",
		name,
		status,
		schedule.CreationTimestamp.Time,
		schedule.Spec.Schedule,
		schedule.Spec.Template.TTL.Duration,
		humanReadableTimeFromNow(schedule.Status.LastBackup.Time),
		metav1.FormatLabelSelector(schedule.Spec.Template.LabelSelector),
	)

	if err != nil {
		return err
	}

	if _, err := fmt.Fprint(w, printers.AppendLabels(schedule.Labels, options.ColumnLabels)); err != nil {
		return err
	}

	_, err = fmt.Fprint(w, printers.AppendAllLabels(options.ShowLabels, schedule.Labels))
	return err
}
