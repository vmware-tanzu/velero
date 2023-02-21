/*
Copyright the Velero contributors.

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
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fatih/color"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/cmd/util/downloadrequest"
	clientset "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned"
	"github.com/vmware-tanzu/velero/pkg/util/results"
)

func DescribeRestore(ctx context.Context, kbClient kbclient.Client, restore *velerov1api.Restore, podVolumeRestores []velerov1api.PodVolumeRestore, details bool, veleroClient clientset.Interface, insecureSkipTLSVerify bool, caCertFile string) string {
	return Describe(func(d *Describer) {
		d.DescribeMetadata(restore.ObjectMeta)

		d.Println()
		phase := restore.Status.Phase
		if phase == "" {
			phase = velerov1api.RestorePhaseNew
		}
		phaseString := string(phase)
		switch phase {
		case velerov1api.RestorePhaseCompleted:
			phaseString = color.GreenString(phaseString)
		case velerov1api.RestorePhaseFailedValidation, velerov1api.RestorePhasePartiallyFailed, velerov1api.RestorePhaseFailed:
			phaseString = color.RedString(phaseString)
		}

		resultsNote := ""
		if phase == velerov1api.RestorePhaseFailed || phase == velerov1api.RestorePhasePartiallyFailed {
			resultsNote = fmt.Sprintf(" (run 'velero restore logs %s' for more information)", restore.Name)
		}

		d.Printf("Phase:\t%s%s\n", phaseString, resultsNote)
		if restore.Status.Progress != nil {
			if restore.Status.Phase == velerov1api.RestorePhaseInProgress {
				d.Printf("Estimated total items to be restored:\t%d\n", restore.Status.Progress.TotalItems)
				d.Printf("Items restored so far:\t%d\n", restore.Status.Progress.ItemsRestored)
			} else {
				d.Printf("Total items to be restored:\t%d\n", restore.Status.Progress.TotalItems)
				d.Printf("Items restored:\t%d\n", restore.Status.Progress.ItemsRestored)
			}
		}

		d.Println()
		// "<n/a>" output should only be applicable for restore that failed validation
		if restore.Status.StartTimestamp == nil || restore.Status.StartTimestamp.IsZero() {
			d.Printf("Started:\t%s\n", "<n/a>")
		} else {
			d.Printf("Started:\t%s\n", restore.Status.StartTimestamp)
		}

		if restore.Status.CompletionTimestamp == nil || restore.Status.CompletionTimestamp.IsZero() {
			d.Printf("Completed:\t%s\n", "<n/a>")
		} else {
			d.Printf("Completed:\t%s\n", restore.Status.CompletionTimestamp)
		}

		if len(restore.Status.ValidationErrors) > 0 {
			d.Println()
			d.Printf("Validation errors:")
			for _, ve := range restore.Status.ValidationErrors {
				d.Printf("\t%s\n", color.RedString(ve))
			}
		}

		describeRestoreResults(ctx, kbClient, d, restore, insecureSkipTLSVerify, caCertFile)

		d.Println()
		d.Printf("Backup:\t%s\n", restore.Spec.BackupName)

		d.Println()
		d.Printf("Namespaces:\n")
		var s string
		if len(restore.Spec.IncludedNamespaces) == 0 {
			s = "all namespaces found in the backup"
		} else if len(restore.Spec.IncludedNamespaces) == 1 && restore.Spec.IncludedNamespaces[0] == "*" {
			s = "all namespaces found in the backup"
		} else {
			s = strings.Join(restore.Spec.IncludedNamespaces, ", ")
		}
		d.Printf("\tIncluded:\t%s\n", s)
		if len(restore.Spec.ExcludedNamespaces) == 0 {
			s = emptyDisplay
		} else {
			s = strings.Join(restore.Spec.ExcludedNamespaces, ", ")
		}
		d.Printf("\tExcluded:\t%s\n", s)

		d.Println()
		d.Printf("Resources:\n")
		if len(restore.Spec.IncludedResources) == 0 {
			s = "*"
		} else {
			s = strings.Join(restore.Spec.IncludedResources, ", ")
		}
		d.Printf("\tIncluded:\t%s\n", s)
		if len(restore.Spec.ExcludedResources) == 0 {
			s = emptyDisplay
		} else {
			s = strings.Join(restore.Spec.ExcludedResources, ", ")
		}
		d.Printf("\tExcluded:\t%s\n", s)

		d.Printf("\tCluster-scoped:\t%s\n", BoolPointerString(restore.Spec.IncludeClusterResources, "excluded", "included", "auto"))

		d.Println()
		d.DescribeMap("Namespace mappings", restore.Spec.NamespaceMapping)

		d.Println()
		s = emptyDisplay
		if restore.Spec.LabelSelector != nil {
			s = metav1.FormatLabelSelector(restore.Spec.LabelSelector)
		}
		d.Printf("Label selector:\t%s\n", s)

		d.Println()
		d.Printf("Restore PVs:\t%s\n", BoolPointerString(restore.Spec.RestorePVs, "false", "true", "auto"))

		if len(podVolumeRestores) > 0 {
			d.Println()
			describePodVolumeRestores(d, podVolumeRestores, details)
		}

		d.Println()
		s = emptyDisplay
		if restore.Spec.ExistingResourcePolicy != "" {
			s = string(restore.Spec.ExistingResourcePolicy)
		}
		d.Printf("Existing Resource Policy: \t%s\n", s)

		d.Println()
		d.Printf("Preserve Service NodePorts:\t%s\n", BoolPointerString(restore.Spec.PreserveNodePorts, "false", "true", "auto"))

	})
}

func describeRestoreResults(ctx context.Context, kbClient kbclient.Client, d *Describer, restore *velerov1api.Restore, insecureSkipTLSVerify bool, caCertPath string) {
	if restore.Status.Warnings == 0 && restore.Status.Errors == 0 {
		return
	}

	var buf bytes.Buffer
	var resultMap map[string]results.Result

	if err := downloadrequest.Stream(ctx, kbClient, restore.Namespace, restore.Name, velerov1api.DownloadTargetKindRestoreResults, &buf, downloadRequestTimeout, insecureSkipTLSVerify, caCertPath); err != nil {
		d.Printf("Warnings:\t<error getting warnings: %v>\n\nErrors:\t<error getting errors: %v>\n", err, err)
		return
	}

	if err := json.NewDecoder(&buf).Decode(&resultMap); err != nil {
		d.Printf("Warnings:\t<error decoding warnings: %v>\n\nErrors:\t<error decoding errors: %v>\n", err, err)
		return
	}

	if restore.Status.Warnings > 0 {
		d.Println()
		describeRestoreResult(d, "Warnings", resultMap["warnings"])
	}
	if restore.Status.Errors > 0 {
		d.Println()
		describeRestoreResult(d, "Errors", resultMap["errors"])
	}
}

func describeRestoreResult(d *Describer, name string, result results.Result) {
	d.Printf("%s:\n", name)
	d.DescribeSlice(1, "Velero", result.Velero)
	d.DescribeSlice(1, "Cluster", result.Cluster)
	if len(result.Namespaces) == 0 {
		d.Printf("\tNamespaces: <none>\n")
	} else {
		d.Printf("\tNamespaces:\n")
		for ns, warnings := range result.Namespaces {
			d.DescribeSlice(2, ns, warnings)
		}
	}
}

// describePodVolumeRestores describes pod volume restores in human-readable format.
func describePodVolumeRestores(d *Describer, restores []velerov1api.PodVolumeRestore, details bool) {
	// Get the type of pod volume uploader. Since the uploader only comes from a single source, we can
	// take the uploader type from the first element of the array.
	var uploaderType string
	if len(restores) > 0 {
		uploaderType = restores[0].Spec.UploaderType
	} else {
		return
	}

	if details {
		d.Printf("%s Restores:\n", uploaderType)
	} else {
		d.Printf("%s Restores (specify --details for more information):\n", uploaderType)
	}

	// separate restores by phase (combining <none> and New into a single group)
	restoresByPhase := groupRestoresByPhase(restores)

	// go through phases in a specific order
	for _, phase := range []string{
		string(velerov1api.PodVolumeRestorePhaseCompleted),
		string(velerov1api.PodVolumeRestorePhaseFailed),
		"In Progress",
		string(velerov1api.PodVolumeRestorePhaseNew),
	} {
		if len(restoresByPhase[phase]) == 0 {
			continue
		}

		// if we're not printing details, just report the phase and count
		if !details {
			d.Printf("\t%s:\t%d\n", phase, len(restoresByPhase[phase]))
			continue
		}

		// group the restores in the current phase by pod (i.e. "ns/name")
		restoresByPod := new(volumesByPod)

		for _, restore := range restoresByPhase[phase] {
			restoresByPod.Add(restore.Spec.Pod.Namespace, restore.Spec.Pod.Name, restore.Spec.Volume, phase, restore.Status.Progress)
		}

		d.Printf("\t%s:\n", phase)
		for _, restoreGroup := range restoresByPod.Sorted() {
			sort.Strings(restoreGroup.volumes)

			// print volumes restored up for this pod
			d.Printf("\t\t%s: %s\n", restoreGroup.label, strings.Join(restoreGroup.volumes, ", "))
		}
	}
}

func groupRestoresByPhase(restores []velerov1api.PodVolumeRestore) map[string][]velerov1api.PodVolumeRestore {
	restoresByPhase := make(map[string][]velerov1api.PodVolumeRestore)

	phaseToGroup := map[velerov1api.PodVolumeRestorePhase]string{
		velerov1api.PodVolumeRestorePhaseCompleted:  string(velerov1api.PodVolumeRestorePhaseCompleted),
		velerov1api.PodVolumeRestorePhaseFailed:     string(velerov1api.PodVolumeRestorePhaseFailed),
		velerov1api.PodVolumeRestorePhaseInProgress: "In Progress",
		velerov1api.PodVolumeRestorePhaseNew:        string(velerov1api.PodVolumeRestorePhaseNew),
		"":                                          string(velerov1api.PodVolumeRestorePhaseNew),
	}

	for _, restore := range restores {
		group := phaseToGroup[restore.Status.Phase]
		restoresByPhase[group] = append(restoresByPhase[group], restore)
	}

	return restoresByPhase
}
