/*
Copyright 2017, 2019 the Velero contributors.

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
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/kubernetes/pkg/printers"

	velerov1api "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/cmd/util/downloadrequest"
	clientset "github.com/heptio/velero/pkg/generated/clientset/versioned"
)

var (
	backupColumns = []string{"NAME", "STATUS", "CREATED", "EXPIRES", "STORAGE LOCATION", "SELECTOR"}
)

func printBackupList(list *velerov1api.BackupList, w io.Writer, options printers.PrintOptions) error {
	sortBackupsByPrefixAndTimestamp(list)

	for i := range list.Items {
		if err := printBackup(&list.Items[i], w, options); err != nil {
			return err
		}
	}
	return nil
}

// sort by default alphabetically, but if backups stem from a common schedule
// (detected by the presence of a 14-digit timestamp suffix), then within that
// group, sort by newest to oldest (i.e. prefix ASC, suffix DESC)
var timestampSuffix = regexp.MustCompile("-[0-9]{14}$")

func sortBackupsByPrefixAndTimestamp(list *velerov1api.BackupList) {

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

func printBackup(backup *velerov1api.Backup, w io.Writer, options printers.PrintOptions) error {
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

	status := string(backup.Status.Phase)
	if status == "" {
		status = string(velerov1api.BackupPhaseNew)
	}
	if backup.DeletionTimestamp != nil && !backup.DeletionTimestamp.Time.IsZero() {
		status = "Deleting"
	}
	if status == string(velerov1api.BackupPhasePartiallyFailed) {
		if backup.Status.Errors == 1 {
			status = fmt.Sprintf("%s (1 error)", status)
		} else {
			status = fmt.Sprintf("%s (%d errors)", status, backup.Status.Errors)
		}

	}

	location := backup.Spec.StorageLocation

	if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s", name, status, backup.Status.StartTimestamp.Time, humanReadableTimeFromNow(expiration), location, metav1.FormatLabelSelector(backup.Spec.LabelSelector)); err != nil {
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
		return duration.ShortHumanDuration(when.Sub(now))
	default:
		return fmt.Sprintf("%s ago", duration.ShortHumanDuration(now.Sub(when)))
	}
}

func printBackupResourceList(d *Describer, backup *velerov1api.Backup, veleroClient clientset.Interface) {
	buf := new(bytes.Buffer)
	if err := downloadrequest.Stream(veleroClient.VeleroV1(), backup.Namespace, backup.Name, velerov1api.DownloadTargetKindBackupResourceList, buf, downloadRequestTimeout); err != nil {
		d.Printf("Resource List:\t<error getting backup resource list: %v>\n", err)
		return
	}

	var resourceList map[string][]string
	if err := json.NewDecoder(buf).Decode(&resourceList); err != nil {
		d.Printf("Resource List:\t<error reading backup resource list: %v>\n", err)
		return
	}

	d.Println("Resource List:")
	for gvk, items := range resourceList {
		d.Printf("\t%s:\n\t\t- %s\n", gvk, strings.Join(items, "\n\t\t- "))
	}
}

func printSnapshot(d *Describer, pvName, snapshotID, volumeType, volumeAZ string, iops *int64) {
	d.Printf("\t%s:\n", pvName)
	d.Printf("\t\tSnapshot ID:\t%s\n", snapshotID)
	d.Printf("\t\tType:\t%s\n", volumeType)
	d.Printf("\t\tAvailability Zone:\t%s\n", volumeAZ)
	iopsString := "<N/A>"
	if iops != nil {
		iopsString = fmt.Sprintf("%d", *iops)
	}
	d.Printf("\t\tIOPS:\t%s\n", iopsString)
}
