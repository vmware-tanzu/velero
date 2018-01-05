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
	"encoding/json"
	"strings"
	"time"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cmd/util/downloadrequest"
	clientset "github.com/heptio/ark/pkg/generated/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DescribeRestore(restore *v1.Restore, arkClient clientset.Interface) string {
	return Describe(func(d *Describer) {
		d.DescribeMetadata(restore.ObjectMeta)

		d.Println()
		d.Printf("Backup:\t%s\n", restore.Spec.BackupName)

		d.Println()
		d.Printf("Namespaces:\n")
		var s string
		if len(restore.Spec.IncludedNamespaces) == 0 {
			s = "*"
		} else {
			s = strings.Join(restore.Spec.IncludedNamespaces, ", ")
		}
		d.Printf("\tIncluded:\t%s\n", s)
		if len(restore.Spec.ExcludedNamespaces) == 0 {
			s = "<none>"
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
			s = "<none>"
		} else {
			s = strings.Join(restore.Spec.ExcludedResources, ", ")
		}
		d.Printf("\tExcluded:\t%s\n", s)

		d.Printf("\tCluster-scoped:\t%s\n", BoolPointerString(restore.Spec.IncludeClusterResources, "excluded", "included", "auto"))

		d.Println()
		d.DescribeMap("Namespace mappings", restore.Spec.NamespaceMapping)

		d.Println()
		s = "<none>"
		if restore.Spec.LabelSelector != nil {
			s = metav1.FormatLabelSelector(restore.Spec.LabelSelector)
		}
		d.Printf("Label selector:\t%s\n", s)

		d.Println()
		d.Printf("Restore PVs:\t%s\n", BoolPointerString(restore.Spec.RestorePVs, "false", "true", "auto"))

		d.Println()
		d.Printf("Phase:\t%s\n", restore.Status.Phase)

		d.Println()
		d.Printf("Validation errors:")
		if len(restore.Status.ValidationErrors) == 0 {
			d.Printf("\t<none>\n")
		} else {
			for _, ve := range restore.Status.ValidationErrors {
				d.Printf("\t%s\n", ve)
			}
		}

		d.Println()
		describeRestoreResults(d, restore, arkClient)
	})
}

func describeRestoreResults(d *Describer, restore *v1.Restore, arkClient clientset.Interface) {
	if restore.Status.Warnings == 0 && restore.Status.Errors == 0 {
		d.Printf("Warnings:\t<none>\nErrors:\t<none>\n")
		return
	}

	var buf bytes.Buffer
	var resultMap map[string]v1.RestoreResult

	if err := downloadrequest.Stream(arkClient.ArkV1(), restore.Namespace, restore.Name, v1.DownloadTargetKindRestoreResults, &buf, 30*time.Second); err != nil {
		d.Printf("Warnings:\t<error getting warnings: %v>\n\nErrors:\t<error getting errors: %v>\n", err, err)
		return
	}

	if err := json.NewDecoder(&buf).Decode(&resultMap); err != nil {
		d.Printf("Warnings:\t<error decoding warnings: %v>\n\nErrors:\t<error decoding errors: %v>\n", err, err)
		return
	}

	describeRestoreResult(d, "Warnings", resultMap["warnings"])
	d.Println()
	describeRestoreResult(d, "Errors", resultMap["errors"])
}

func describeRestoreResult(d *Describer, name string, result v1.RestoreResult) {
	d.Printf("%s:\n", name)
	d.DescribeSlice(1, "Ark", result.Ark)
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
