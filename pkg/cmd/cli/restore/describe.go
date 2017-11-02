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

package restore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	clientset "github.com/heptio/ark/pkg/generated/clientset/versioned"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cmd"
	"github.com/heptio/ark/pkg/cmd/util/downloadrequest"
	"github.com/heptio/ark/pkg/cmd/util/output"
)

func NewDescribeCommand(f client.Factory, use string) *cobra.Command {
	var listOptions metav1.ListOptions

	c := &cobra.Command{
		Use:   use,
		Short: "Describe restores",
		Run: func(c *cobra.Command, args []string) {
			arkClient, err := f.Client()
			cmd.CheckError(err)

			var restores *api.RestoreList
			if len(args) > 0 {
				restores = new(api.RestoreList)
				for _, name := range args {
					restore, err := arkClient.Ark().Restores(api.DefaultNamespace).Get(name, metav1.GetOptions{})
					cmd.CheckError(err)
					restores.Items = append(restores.Items, *restore)
				}
			} else {
				restores, err = arkClient.ArkV1().Restores(api.DefaultNamespace).List(listOptions)
				cmd.CheckError(err)
			}

			first := true
			for _, restore := range restores.Items {
				s := output.Describe(func(out io.Writer) {
					describeRestore(out, &restore, arkClient)
				})
				if first {
					first = false
					fmt.Print(s)
				} else {
					fmt.Printf("\n\n%s", s)
				}
			}
			cmd.CheckError(err)
		},
	}

	c.Flags().StringVarP(&listOptions.LabelSelector, "selector", "l", listOptions.LabelSelector, "only show items matching this label selector")

	output.BindFlags(c.Flags())

	return c
}

func describeRestore(out io.Writer, restore *api.Restore, arkClient clientset.Interface) {
	output.DescribeMetadata(out, restore.ObjectMeta)

	fmt.Fprintln(out)
	fmt.Fprintf(out, "Backup:\t%s\n", restore.Spec.BackupName)

	fmt.Fprintln(out)
	fmt.Fprintf(out, "Namespaces:\n")
	var s string
	if len(restore.Spec.IncludedNamespaces) == 0 {
		s = "*"
	} else {
		s = strings.Join(restore.Spec.IncludedNamespaces, ", ")
	}
	fmt.Fprintf(out, "\tIncluded:\t%s\n", s)
	if len(restore.Spec.ExcludedNamespaces) == 0 {
		s = "<none>"
	} else {
		s = strings.Join(restore.Spec.ExcludedNamespaces, ", ")
	}
	fmt.Fprintf(out, "\tExcluded:\t%s\n", s)

	fmt.Fprintln(out)
	fmt.Fprintf(out, "Resources:\n")
	if len(restore.Spec.IncludedResources) == 0 {
		s = "*"
	} else {
		s = strings.Join(restore.Spec.IncludedResources, ", ")
	}
	fmt.Fprintf(out, "\tIncluded:\t%s\n", s)
	if len(restore.Spec.ExcludedResources) == 0 {
		s = "<none>"
	} else {
		s = strings.Join(restore.Spec.ExcludedResources, ", ")
	}
	fmt.Fprintf(out, "\tExcluded:\t%s\n", s)

	fmt.Fprintf(out, "\tCluster-scoped:\t%s\n", output.BoolPointerString(restore.Spec.IncludeClusterResources, "excluded", "included", "auto"))

	fmt.Fprintln(out)
	output.DescribeMap(out, "Namespace mappings", restore.Spec.NamespaceMapping)

	fmt.Fprintln(out)
	s = "<none>"
	if restore.Spec.LabelSelector != nil {
		s = metav1.FormatLabelSelector(restore.Spec.LabelSelector)
	}
	fmt.Fprintf(out, "Label selector:\t%s\n", s)

	fmt.Fprintln(out)
	fmt.Fprintf(out, "Restore PVs:\t%s\n", output.BoolPointerString(restore.Spec.RestorePVs, "false", "true", "auto"))

	fmt.Fprintln(out)
	fmt.Fprintf(out, "Phase:\t%s\n", restore.Status.Phase)

	fmt.Fprintln(out)
	fmt.Fprint(out, "Validation errors:")
	if len(restore.Status.ValidationErrors) == 0 {
		fmt.Fprintf(out, "\t<none>\n")
	} else {
		for _, ve := range restore.Status.ValidationErrors {
			fmt.Fprintf(out, "\t%s\n", ve)
		}
	}

	fmt.Fprintln(out)
	describeRestoreResults(out, arkClient, restore)
}

func describeRestoreResults(out io.Writer, arkClient clientset.Interface, restore *api.Restore) {
	if restore.Status.Warnings == 0 && restore.Status.Errors == 0 {
		fmt.Fprintf(out, "Warnings:\t<none>\nErrors:\t<none>\n")
		return
	}

	var buf bytes.Buffer
	var resultMap map[string]api.RestoreResult

	if err := downloadrequest.Stream(arkClient.ArkV1(), restore.Name, api.DownloadTargetKindRestoreResults, &buf, 30*time.Second); err != nil {
		fmt.Fprintf(out, "Warnings:\t<error getting warnings: %v>\n\nErrors:\t<error getting errors: %v>\n", err, err)
		return
	}

	if err := json.NewDecoder(&buf).Decode(&resultMap); err != nil {
		fmt.Fprintf(out, "Warnings:\t<error decoding warnings: %v>\n\nErrors:\t<error decoding errors: %v>\n", err, err)
		return
	}

	describeRestoreResult(out, "Warnings", resultMap["warnings"])
	fmt.Fprintln(out)
	describeRestoreResult(out, "Errors", resultMap["errors"])
}

func describeRestoreResult(out io.Writer, name string, result api.RestoreResult) {
	fmt.Fprintf(out, "%s:\n", name)
	output.DescribeSlice(out, 1, "Ark", result.Ark)
	output.DescribeSlice(out, 1, "Cluster", result.Cluster)
	if len(result.Namespaces) == 0 {
		fmt.Fprintf(out, "\tNamespaces: <none>\n")
	} else {
		fmt.Fprintf(out, "\tNamespaces:\n")
		for ns, warnings := range result.Namespaces {
			output.DescribeSlice(out, 2, ns, warnings)
		}
	}
}
