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
	"regexp"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/printers"

	"github.com/heptio/ark/pkg/apis/ark/v1"
)

var (
	backupColumns = []string{"NAME", "STATUS", "CREATED", "EXPIRES", "SELECTOR"}
)

func printBackupList(list *v1.BackupList, w io.Writer, options printers.PrintOptions) error {
	sortBackupsByPrefixAndTimestamp(list)

	for i := range list.Items {
		if err := printBackup(&list.Items[i], w, options); err != nil {
			return err
		}
	}
	return nil
}

func sortBackupsByPrefixAndTimestamp(list *v1.BackupList) {
	// sort by default alphabetically, but if backups stem from a common schedule
	// (detected by the presence of a 14-digit timestamp suffix), then within that
	// group, sort by newest to oldest (i.e. prefix ASC, suffix DESC)
	timestampSuffix := regexp.MustCompile("-[0-9]{14}$")

	sort.Slice(list.Items, func(i, j int) bool {
		iSuffixIndex := timestampSuffix.FindStringIndex(list.Items[i].Name)
		jSuffixIndex := timestampSuffix.FindStringIndex(list.Items[j].Name)

		// one/both don't have a timestamp suffix, so sort alphabetically
		if iSuffixIndex == nil || jSuffixIndex == nil {
			return list.Items[i].Name < list.Items[j].Name
		}

		// different prefixes, so sort alphabetically
		if list.Items[i].Name[0:iSuffixIndex[0]] != list.Items[j].Name[0:jSuffixIndex[0]] {
			return list.Items[i].Name < list.Items[j].Name
		}

		// same prefixes, so sort based on suffix (desc)
		return list.Items[i].Name[iSuffixIndex[0]:] >= list.Items[j].Name[jSuffixIndex[0]:]
	})
}

func printBackup(backup *v1.Backup, w io.Writer, options printers.PrintOptions) error {
	name := printers.FormatResourceName(options.Kind, backup.Name, options.WithKind)

	if options.WithNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", backup.Namespace); err != nil {
			return err
		}
	}

	expiration := backup.Status.Expiration.Time
	if expiration.IsZero() && backup.Spec.TTL.Duration > 0 {
		expiration = backup.CreationTimestamp.Add(backup.Spec.TTL.Duration)
	}

	status := backup.Status.Phase
	if status == "" {
		status = v1.BackupPhaseNew
	}

	if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s", name, status, backup.CreationTimestamp.Time, humanReadableTimeFromNow(expiration), metav1.FormatLabelSelector(backup.Spec.LabelSelector)); err != nil {
		return err
	}

	if _, err := fmt.Fprint(w, printers.AppendLabels(backup.Labels, options.ColumnLabels)); err != nil {
		return err
	}

	_, err := fmt.Fprint(w, printers.AppendAllLabels(options.ShowLabels, backup.Labels))
	return err
}

func humanReadableTimeFromNow(when time.Time) string {
	if when.IsZero() {
		return "n/a"
	}

	now := time.Now()
	switch {
	case when == now || when.After(now):
		return printers.ShortHumanDuration(when.Sub(now))
	default:
		return fmt.Sprintf("%s ago", printers.ShortHumanDuration(now.Sub(when)))
	}
}
