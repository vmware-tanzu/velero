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

package restore

import (
	"context"
	"sort"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/archive"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/features"
)

// meetsAPIGVRestoreReqs determines if the feature flag has been
// enabled and the backup objects use Status.FormatVersion 1.1.0.
func (ctx *restoreContext) meetsAPIGVRestoreReqs() bool {
	return features.IsEnabled(velerov1api.APIGroupVersionsFeatureFlag) &&
		ctx.backup.Status.FormatVersion == "1.1.0"
}

// ChosenGroupVersion is the API Group version that was selected to restore
// from potentially multiple backed up version enabled by the feature flag
// APIGroupVersionsFeatureFlag
type ChosenGroupVersion struct {
	Group   string
	Version string
	Dir     string
}

// chooseAPIVersionsToRestore will choose a version to restore based on a user-
// provided config map prioritization or our version prioritization.
func (ctx *restoreContext) chooseAPIVersionsToRestore() error {
	sgvs, tgvs, ugvs, err := ctx.gatherSTUVersions()
	if err != nil {
		return err
	}

OUTER:
	for rg, sg := range sgvs {
		// Default to the source preferred version if no other common version
		// can be found.
		cgv := ChosenGroupVersion{
			Group:   sg.Name,
			Version: sg.PreferredVersion.Version,
			Dir:     sg.PreferredVersion.Version + "-preferredversion",
		}

		tg := findTargetGroup(tgvs, sg.Name)
		if len(tg.Versions) == 0 {
			ctx.SetChosenGVToRestore(cgv, rg, "", "")
			continue
		}

		// Priority 0: User Priority Version
		if ugvs != nil {
			uv := findSupportedUserVersion(ugvs[rg].Versions, tg.Versions, sg.Versions)
			if uv != "" {
				ctx.SetChosenGVToRestore(cgv, rg, sg.PreferredVersion.Version, uv)
				ctx.log.Debugf("APIGroupVersionsFeatureFlag Priority 0: User defined API group version %s chosen for %s", uv, rg)
				continue
			}

			ctx.log.Infof("Cannot find user defined version in both the cluster and backup cluster. Ignoring version %s for %s", uv, rg)
		}

		// Priority 1: Target Cluster Preferred Version
		if versionsContain(sg.Versions, tg.PreferredVersion.Version) {
			ctx.SetChosenGVToRestore(
				cgv,
				rg,
				sg.PreferredVersion.Version,
				tg.PreferredVersion.Version,
			)
			ctx.log.Debugf(
				"APIGroupVersionsFeatureFlag Priority 1: Cluster preferred API group version %s found in backup for %s",
				tg.PreferredVersion.Version,
				rg,
			)
			continue
		}

		// Priority 2: Source Cluster Preferred Version
		if versionsContain(tg.Versions, sg.PreferredVersion.Version) {
			ctx.SetChosenGVToRestore(
				cgv,
				rg,
				sg.PreferredVersion.Version,
				sg.PreferredVersion.Version,
			)
			ctx.log.Debugf(
				"APIGroupVersionsFeatureFlag Priority 2: Cluster preferred API group version not found in backup. Using backup preferred version %s for %s",
				sg.PreferredVersion.Version,
				rg,
			)
			continue
		}

		// Priority 3: Kubernetes Prioritized Common Supported Version
		for _, tv := range tg.Versions[1:] {
			if versionsContain(sg.Versions[1:], tv.Version) {
				ctx.SetChosenGVToRestore(cgv, rg, sg.PreferredVersion.Version, tv.Version)
				continue OUTER
			}
			ctx.log.Debugf(
				"APIGroupVersionsFeatureFlag Priority 3: Common supported but not preferred API group version %s chosen for %s",
				tv.Version,
				rg,
			)
		}

		// Use default group version.
		ctx.SetChosenGVToRestore(cgv, rg, "", "")
		ctx.log.Debugf(
			"APIGroupVersionsFeatureFlag: Unable to find supported priority API group version. Using backup preferred version %s for %s (default behavior without feature flag).",
			tg.PreferredVersion.Version,
			rg,
		)
	}

	return nil
}

// gatherSTUVersions collects the source, target, and user priority versions.
func (ctx *restoreContext) gatherSTUVersions() (
	map[string]metav1.APIGroup,
	[]metav1.APIGroup,
	map[string]metav1.APIGroup,
	error,
) {
	sourceRGVersions, err := archive.NewParser(ctx.log, ctx.fileSystem).ParseGroupVersions(ctx.restoreDir)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "parsing versions from directory names")
	}

	// Sort the versions in the APIGroups in sourceRGVersions map values.
	for _, src := range sourceRGVersions {
		k8sPrioritySort(src.Versions)
	}

	targetGroupVersions := ctx.discoveryHelper.APIGroups()

	// Sort the versions in the APIGroups slice in targetGroupVersions.
	for _, tar := range targetGroupVersions {
		k8sPrioritySort(tar.Versions)
	}

	// Get the user-provided enableapigroupversion config map.
	cm, err := userPriorityConfigMap()
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "retrieving enableapigroupversion config map")
	}

	// Read user-defined version priorities from config map.
	userRGVPriorities := userResourceGroupVersionPriorities(ctx, cm)

	return sourceRGVersions, targetGroupVersions, userRGVPriorities, nil
}

// k8sPrioritySort sorts slices using Kubernetes' version prioritization.
func k8sPrioritySort(gvs []metav1.GroupVersionForDiscovery) {
	sort.SliceStable(gvs, func(i, j int) bool {
		return version.CompareKubeAwareVersionStrings(gvs[i].Version, gvs[j].Version) > 0
	})
}

// userResourceGroupVersionPriorities retrieves a user-provided config map and
// extracts the user priority versions for each resource.
func userResourceGroupVersionPriorities(ctx *restoreContext, cm *corev1.ConfigMap) map[string]metav1.APIGroup {
	if cm == nil {
		ctx.log.Debugf("No enableapigroupversion config map found in velero namespace. Using pre-defined priorities.")
		return nil
	}

	priorities := parseUserPriorities(cm.Data["restoreResourcesVersionPriority"])
	if len(priorities) == 0 {
		ctx.log.Debugf("No valid user version priorities found in enableapigroupversion config map. Using pre-defined priorities.")
		return nil
	}

	return priorities
}

func userPriorityConfigMap() (*corev1.ConfigMap, error) {
	cfg, err := client.LoadConfig()
	if err != nil {
		return nil, errors.Wrap(err, "reading client config file")
	}

	fc := client.NewFactory("APIGroupVersionsRestore", cfg)

	kc, err := fc.KubeClient()
	if err != nil {
		return nil, errors.Wrap(err, "getting Kube client")
	}

	cm, err := kc.CoreV1().ConfigMaps("velero").Get(
		context.Background(),
		"enableapigroupversions",
		metav1.GetOptions{},
	)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, errors.Wrap(err, "getting enableapigroupversions config map from velero namespace")
	}

	return cm, nil
}

func parseUserPriorities(prioritiesData string) map[string]metav1.APIGroup {
	ups := make(map[string]metav1.APIGroup)

	// The user priorities will be in a string of the form
	// rockbands.music.example.io=v2beta1,v2beta2\n
	// orchestras.music.example.io=v2,v3alpha1\n
	// subscriptions.operators.coreos.com=v2,v1

	lines := strings.Split(prioritiesData, "\n")

	for _, l := range lines {
		if !isUserPriorityValid(l) {
			continue
		}

		rgvs := strings.SplitN(l, "=", 2)
		rg := rgvs[0]   // rockbands.music.example.io
		vers := rgvs[1] // v2beta1,v2beta2

		vs := strings.Split(vers, ",")

		ups[rg] = metav1.APIGroup{
			Versions: versionsToGroupVersionForDiscovery(vs),
		}
	}

	return ups
}

func isUserPriorityValid(line string) bool {
	// Line must have one and only one equal sign
	if strings.Count(line, "=") != 1 {
		return false
	}

	// Line must contain at least one character before and after equal sign
	pair := strings.Split(line, "=")
	if len(pair[0]) < 1 || len(pair[1]) < 1 {
		return false
	}

	// Line must not contain any spaces
	if strings.Count(line, " ") > 0 {
		return false
	}

	return true
}

// versionsToGroupVersionForDiscovery converts version strings into a Kubernetes format
// for group versions.
func versionsToGroupVersionForDiscovery(vs []string) []metav1.GroupVersionForDiscovery {
	gvs := make([]metav1.GroupVersionForDiscovery, len(vs))

	for i, v := range vs {
		gvs[i] = metav1.GroupVersionForDiscovery{
			Version: v,
		}
	}

	return gvs
}

// findTargetGroup looks for a particular group in from a list of groups in the
// target cluster.
func findTargetGroup(tgvs []metav1.APIGroup, group string) metav1.APIGroup {
	for _, tg := range tgvs {
		if tg.Name == group {
			return tg
		}
	}

	return metav1.APIGroup{}
}

// findSupportedUserVersion finds the first user priority version that both source
// and target support.
func findSupportedUserVersion(ugvs, tgvs, sgvs []metav1.GroupVersionForDiscovery) string {
	for _, ug := range ugvs {
		if versionsContain(tgvs, ug.Version) && versionsContain(sgvs, ug.Version) {
			return ug.Version
		}
	}

	return ""
}

// versionsContain will check if a version can be found in a a slice of versions.
func versionsContain(list []metav1.GroupVersionForDiscovery, version string) bool {
	for _, v := range list {
		if v.Version == version {
			return true
		}
	}

	return false
}

// latestCommon will find the versions that are in common between the source and
// target clusters. It will return the latest of the common versions.
func latestCommon(sgvs, tgvs []metav1.GroupVersionForDiscovery) string {
	// Sort target and source versions each in descending lexicographical order.
	// The first matching versions will be the latest.
	sort.SliceStable(sgvs, func(i, j int) bool {
		return strings.Compare(sgvs[i].Version, sgvs[j].Version) > 0
	})
	sort.SliceStable(tgvs, func(i, j int) bool {
		return strings.Compare(tgvs[i].Version, tgvs[j].Version) > 0
	})

	for _, s := range sgvs {
		for _, t := range tgvs {
			if s.Version == t.Version {
				return s.Version
			}
		}
	}

	return ""
}

func (ctx *restoreContext) SetChosenGVToRestore(cgv ChosenGroupVersion, rg, srcPreferred, chosen string) {
	// If the chosen version isn't empty, update the default.
	if chosen != "" {
		cgv.Version = chosen

		cgv.Dir = cgv.Version
		if chosen == srcPreferred {
			cgv.Dir = chosen + "-preferredversion"
		}
	}

	ctx.chosenGrpVersToRestore[rg] = cgv

	ctx.log.Debugf("Chose %s/%s API group version to restore", cgv.Group, cgv.Version)
}
