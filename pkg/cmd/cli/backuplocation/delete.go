/*
Copyright 2020 the Velero contributors.

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

package backuplocation

import (
        "context"
        "fmt"

        "github.com/pkg/errors"
        "github.com/spf13/cobra"
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
        "k8s.io/apimachinery/pkg/labels"
        kubeerrs "k8s.io/apimachinery/pkg/util/errors"

        velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
        "github.com/vmware-tanzu/velero/pkg/client"
        "github.com/vmware-tanzu/velero/pkg/cmd"
        "github.com/vmware-tanzu/velero/pkg/cmd/cli"
)

// NewDeleteCommand creates and returns a new cobra command for deleting backup-locations.
func NewDeleteCommand(f client.Factory, use string) *cobra.Command {
        o := cli.NewDeleteOptions("backup-location")

        c := &cobra.Command{
                Use:   fmt.Sprintf("%s [NAMES]", use),
                Short: "Delete backup storage locations",
                Example: `      # Delete a backup storage location named "backup-location-1."
                velero backup-location delete backup-location-1
        
                # Delete a backup storage location named "backup-location-1" without prompting for confirmation.
                velero backup-location delete backup-location-1 --confirm
        
                # Delete backup storage locations named "backup-location-1" and "backup-location-2."
                velero backup-location delete backup-location-1 backup-location-2
        
                # Delete all backup storage locations labelled with foo=bar."
                velero backup-location delete --selector foo=bar
                
                # Delete all backup storage locations.
                velero backup-location delete --all`,

                Run: func(c *cobra.Command, args []string) {
                        cmd.CheckError(o.Complete(f, args))
                        cmd.CheckError(o.Validate(c, f, args))
                        cmd.CheckError(Run(o))
                },
        }

        o.BindFlags(c.Flags())
        return c
}

// Run performs the delete backup-location operation.
func Run(o *cli.DeleteOptions) error {
        if !o.Confirm && !cli.GetConfirmation() {
                // Don't do anything unless we get confirmation
                return nil
        }

        var (
                backupLocations []*velerov1api.BackupStorageLocation
                errs    []error
        )

        switch {
        case len(o.Names) > 0:
                for _, name := range o.Names {
                        backupLocation, err := o.Client.VeleroV1().BackupStorageLocations(o.Namespace).Get(context.TODO(), name, metav1.GetOptions{})
                        if err != nil {
                                errs = append(errs, errors.WithStack(err))
                                continue
                        }
                        backupLocations = append(backupLocations, backupLocation)
                }
        default:
                selector := labels.Everything().String()
                if o.Selector.LabelSelector != nil {
                        selector = o.Selector.String()
                }
                res, err := o.Client.VeleroV1().BackupStorageLocations(o.Namespace).List(context.TODO(), metav1.ListOptions{
                        LabelSelector: selector,
                })
                if err != nil {
                        errs = append(errs, errors.WithStack(err))
                }

                for i := range res.Items {
                        backupLocations = append(backupLocations, &res.Items[i])
                }
        }
        if len(backupLocations) == 0 {
                fmt.Println("No backup-locations found")
                return nil
        }

        for _, s := range backupLocations {
                err := o.Client.VeleroV1().BackupStorageLocations(s.Namespace).Delete(context.TODO(), s.Name, metav1.DeleteOptions{})
                if err != nil {
                        errs = append(errs, errors.WithStack(err))
                        continue
                }
                fmt.Printf("Backup-location deleted: %v\n", s.Name)
        }
        return kubeerrs.NewAggregate(errs)
}

