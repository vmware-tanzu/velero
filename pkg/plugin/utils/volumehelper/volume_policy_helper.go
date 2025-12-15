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

package volumehelper

import (
	"context"

	"github.com/sirupsen/logrus"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/internal/resourcepolicies"
	"github.com/vmware-tanzu/velero/internal/volumehelper"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
)

// ShouldPerformSnapshotWithBackup is used for third-party plugins.
// It supports to check whether the PVC or PodVolume should be backed
// up on demand. On the other hand, the volumeHelperImpl assume there
// is a VolumeHelper instance initialized before calling the
// ShouldPerformXXX functions.
func ShouldPerformSnapshotWithBackup(
	unstructured runtime.Unstructured,
	groupResource schema.GroupResource,
	backup velerov1api.Backup,
	crClient crclient.Client,
	logger logrus.FieldLogger,
) (bool, error) {
	return ShouldPerformSnapshotWithVolumeHelper(
		unstructured,
		groupResource,
		backup,
		crClient,
		logger,
		nil, // no cached VolumeHelper, will create one
	)
}

// ShouldPerformSnapshotWithVolumeHelper is like ShouldPerformSnapshotWithBackup
// but accepts an optional VolumeHelper. If vh is non-nil, it will be used directly,
// avoiding the overhead of creating a new VolumeHelper on each call.
// This is useful for BIA plugins that process multiple PVCs during a single backup
// and want to reuse the same VolumeHelper (with its internal cache) across calls.
func ShouldPerformSnapshotWithVolumeHelper(
	unstructured runtime.Unstructured,
	groupResource schema.GroupResource,
	backup velerov1api.Backup,
	crClient crclient.Client,
	logger logrus.FieldLogger,
	vh volumehelper.VolumeHelper,
) (bool, error) {
	// If a VolumeHelper is provided, use it directly
	if vh != nil {
		return vh.ShouldPerformSnapshot(unstructured, groupResource)
	}

	// Otherwise, create a new VolumeHelper (original behavior for third-party plugins)
	resourcePolicies, err := resourcepolicies.GetResourcePoliciesFromBackup(
		backup,
		crClient,
		logger,
	)
	if err != nil {
		return false, err
	}

	volumeHelperImpl := volumehelper.NewVolumeHelperImpl(
		resourcePolicies,
		backup.Spec.SnapshotVolumes,
		logger,
		crClient,
		boolptr.IsSetToTrue(backup.Spec.DefaultVolumesToFsBackup),
		true,
	)

	return volumeHelperImpl.ShouldPerformSnapshot(unstructured, groupResource)
}

// NewVolumeHelperForBackup creates a VolumeHelper for the given backup with a PVC-to-Pod cache.
// The cache is built for the provided namespaces list to avoid O(N*M) complexity when there
// are many PVCs and pods. See issue #9179 for details.
//
// This function is intended for BIA plugins to create a VolumeHelper once and reuse it
// across multiple Execute() calls for the same backup.
//
// If namespaces is nil or empty, the function will resolve the namespace list from the backup spec.
// If backup.Spec.IncludedNamespaces is empty (meaning all namespaces), it will list all namespaces
// from the cluster.
func NewVolumeHelperForBackup(
	backup velerov1api.Backup,
	crClient crclient.Client,
	logger logrus.FieldLogger,
	namespaces []string,
) (volumehelper.VolumeHelper, error) {
	// If no namespaces provided, resolve from backup spec
	if len(namespaces) == 0 {
		var err error
		namespaces, err = resolveNamespacesForBackup(backup, crClient)
		if err != nil {
			logger.WithError(err).Warn("Failed to resolve namespaces for cache, proceeding without cache")
			namespaces = nil
		}
	}

	resourcePolicies, err := resourcepolicies.GetResourcePoliciesFromBackup(
		backup,
		crClient,
		logger,
	)
	if err != nil {
		return nil, err
	}

	return volumehelper.NewVolumeHelperImplWithNamespaces(
		resourcePolicies,
		backup.Spec.SnapshotVolumes,
		logger,
		crClient,
		boolptr.IsSetToTrue(backup.Spec.DefaultVolumesToFsBackup),
		true,
		namespaces,
	)
}

// resolveNamespacesForBackup determines which namespaces will be backed up.
// If IncludedNamespaces is specified, it returns those (excluding any in ExcludedNamespaces).
// If IncludedNamespaces is empty (meaning all namespaces), it lists all namespaces from the cluster.
func resolveNamespacesForBackup(backup velerov1api.Backup, crClient crclient.Client) ([]string, error) {
	// If specific namespaces are included, use those
	if len(backup.Spec.IncludedNamespaces) > 0 {
		// Filter out excluded namespaces
		excludeSet := make(map[string]bool)
		for _, ns := range backup.Spec.ExcludedNamespaces {
			excludeSet[ns] = true
		}

		var namespaces []string
		for _, ns := range backup.Spec.IncludedNamespaces {
			if !excludeSet[ns] {
				namespaces = append(namespaces, ns)
			}
		}
		return namespaces, nil
	}

	// IncludedNamespaces is empty, meaning all namespaces - list from cluster
	nsList := &corev1api.NamespaceList{}
	if err := crClient.List(context.Background(), nsList); err != nil {
		return nil, err
	}

	// Filter out excluded namespaces
	excludeSet := make(map[string]bool)
	for _, ns := range backup.Spec.ExcludedNamespaces {
		excludeSet[ns] = true
	}

	var namespaces []string
	for _, ns := range nsList.Items {
		if !excludeSet[ns.Name] {
			namespaces = append(namespaces, ns.Name)
		}
	}

	return namespaces, nil
}
