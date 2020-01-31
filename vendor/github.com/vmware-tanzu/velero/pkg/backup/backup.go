/*
Copyright 2017 the Velero contributors.

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

package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/discovery"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/podexec"
	"github.com/vmware-tanzu/velero/pkg/restic"
	"github.com/vmware-tanzu/velero/pkg/util/collections"
)

// BackupVersion is the current backup version for Velero.
const BackupVersion = 1

// Backupper performs backups.
type Backupper interface {
	// Backup takes a backup using the specification in the api.Backup and writes backup and log data
	// to the given writers.
	Backup(logger logrus.FieldLogger, backup *Request, backupFile io.Writer, actions []velero.BackupItemAction, volumeSnapshotterGetter VolumeSnapshotterGetter) error
}

// kubernetesBackupper implements Backupper.
type kubernetesBackupper struct {
	dynamicFactory         client.DynamicFactory
	discoveryHelper        discovery.Helper
	podCommandExecutor     podexec.PodCommandExecutor
	groupBackupperFactory  groupBackupperFactory
	resticBackupperFactory restic.BackupperFactory
	resticTimeout          time.Duration
}

type resolvedAction struct {
	velero.BackupItemAction

	resourceIncludesExcludes  *collections.IncludesExcludes
	namespaceIncludesExcludes *collections.IncludesExcludes
	selector                  labels.Selector
}

func (i *itemKey) String() string {
	return fmt.Sprintf("resource=%s,namespace=%s,name=%s", i.resource, i.namespace, i.name)
}

func cohabitatingResources() map[string]*cohabitatingResource {
	return map[string]*cohabitatingResource{
		"deployments":     newCohabitatingResource("deployments", "extensions", "apps"),
		"daemonsets":      newCohabitatingResource("daemonsets", "extensions", "apps"),
		"replicasets":     newCohabitatingResource("replicasets", "extensions", "apps"),
		"networkpolicies": newCohabitatingResource("networkpolicies", "extensions", "networking.k8s.io"),
		"events":          newCohabitatingResource("events", "", "events.k8s.io"),
	}
}

// NewKubernetesBackupper creates a new kubernetesBackupper.
func NewKubernetesBackupper(
	discoveryHelper discovery.Helper,
	dynamicFactory client.DynamicFactory,
	podCommandExecutor podexec.PodCommandExecutor,
	resticBackupperFactory restic.BackupperFactory,
	resticTimeout time.Duration,
) (Backupper, error) {
	return &kubernetesBackupper{
		discoveryHelper:        discoveryHelper,
		dynamicFactory:         dynamicFactory,
		podCommandExecutor:     podCommandExecutor,
		groupBackupperFactory:  &defaultGroupBackupperFactory{},
		resticBackupperFactory: resticBackupperFactory,
		resticTimeout:          resticTimeout,
	}, nil
}

func resolveActions(actions []velero.BackupItemAction, helper discovery.Helper) ([]resolvedAction, error) {
	var resolved []resolvedAction

	for _, action := range actions {
		resourceSelector, err := action.AppliesTo()
		if err != nil {
			return nil, err
		}

		resources := getResourceIncludesExcludes(helper, resourceSelector.IncludedResources, resourceSelector.ExcludedResources)
		namespaces := collections.NewIncludesExcludes().Includes(resourceSelector.IncludedNamespaces...).Excludes(resourceSelector.ExcludedNamespaces...)

		selector := labels.Everything()
		if resourceSelector.LabelSelector != "" {
			if selector, err = labels.Parse(resourceSelector.LabelSelector); err != nil {
				return nil, err
			}
		}

		res := resolvedAction{
			BackupItemAction:          action,
			resourceIncludesExcludes:  resources,
			namespaceIncludesExcludes: namespaces,
			selector:                  selector,
		}

		resolved = append(resolved, res)
	}

	return resolved, nil
}

// getResourceIncludesExcludes takes the lists of resources to include and exclude, uses the
// discovery helper to resolve them to fully-qualified group-resource names, and returns an
// IncludesExcludes list.
func getResourceIncludesExcludes(helper discovery.Helper, includes, excludes []string) *collections.IncludesExcludes {
	resources := collections.GenerateIncludesExcludes(
		includes,
		excludes,
		func(item string) string {
			gvr, _, err := helper.ResourceFor(schema.ParseGroupResource(item).WithVersion(""))
			if err != nil {
				return ""
			}

			gr := gvr.GroupResource()
			return gr.String()
		},
	)

	return resources
}

// getNamespaceIncludesExcludes returns an IncludesExcludes list containing which namespaces to
// include and exclude from the backup.
func getNamespaceIncludesExcludes(backup *api.Backup) *collections.IncludesExcludes {
	return collections.NewIncludesExcludes().Includes(backup.Spec.IncludedNamespaces...).Excludes(backup.Spec.ExcludedNamespaces...)
}

func getResourceHooks(hookSpecs []api.BackupResourceHookSpec, discoveryHelper discovery.Helper) ([]resourceHook, error) {
	resourceHooks := make([]resourceHook, 0, len(hookSpecs))

	for _, s := range hookSpecs {
		h, err := getResourceHook(s, discoveryHelper)
		if err != nil {
			return []resourceHook{}, err
		}

		resourceHooks = append(resourceHooks, h)
	}

	return resourceHooks, nil
}

func getResourceHook(hookSpec api.BackupResourceHookSpec, discoveryHelper discovery.Helper) (resourceHook, error) {
	h := resourceHook{
		name:       hookSpec.Name,
		namespaces: collections.NewIncludesExcludes().Includes(hookSpec.IncludedNamespaces...).Excludes(hookSpec.ExcludedNamespaces...),
		resources:  getResourceIncludesExcludes(discoveryHelper, hookSpec.IncludedResources, hookSpec.ExcludedResources),
		pre:        hookSpec.PreHooks,
		post:       hookSpec.PostHooks,
	}

	if hookSpec.LabelSelector != nil {
		labelSelector, err := metav1.LabelSelectorAsSelector(hookSpec.LabelSelector)
		if err != nil {
			return resourceHook{}, errors.WithStack(err)
		}
		h.labelSelector = labelSelector
	}

	return h, nil
}

type VolumeSnapshotterGetter interface {
	GetVolumeSnapshotter(name string) (velero.VolumeSnapshotter, error)
}

// Backup backs up the items specified in the Backup, placing them in a gzip-compressed tar file
// written to backupFile. The finalized api.Backup is written to metadata. Any error that represents
// a complete backup failure is returned. Errors that constitute partial failures (i.e. failures to
// back up individual resources that don't prevent the backup from continuing to be processed) are logged
// to the backup log.
func (kb *kubernetesBackupper) Backup(log logrus.FieldLogger, backupRequest *Request, backupFile io.Writer, actions []velero.BackupItemAction, volumeSnapshotterGetter VolumeSnapshotterGetter) error {
	gzippedData := gzip.NewWriter(backupFile)
	defer gzippedData.Close()

	tw := tar.NewWriter(gzippedData)
	defer tw.Close()

	log.Info("Writing backup version file")
	if err := kb.writeBackupVersion(tw); err != nil {
		return errors.WithStack(err)
	}

	backupRequest.NamespaceIncludesExcludes = getNamespaceIncludesExcludes(backupRequest.Backup)
	log.Infof("Including namespaces: %s", backupRequest.NamespaceIncludesExcludes.IncludesString())
	log.Infof("Excluding namespaces: %s", backupRequest.NamespaceIncludesExcludes.ExcludesString())

	backupRequest.ResourceIncludesExcludes = getResourceIncludesExcludes(kb.discoveryHelper, backupRequest.Spec.IncludedResources, backupRequest.Spec.ExcludedResources)
	log.Infof("Including resources: %s", backupRequest.ResourceIncludesExcludes.IncludesString())
	log.Infof("Excluding resources: %s", backupRequest.ResourceIncludesExcludes.ExcludesString())

	var err error
	backupRequest.ResourceHooks, err = getResourceHooks(backupRequest.Spec.Hooks.Resources, kb.discoveryHelper)
	if err != nil {
		return err
	}

	backupRequest.ResolvedActions, err = resolveActions(actions, kb.discoveryHelper)
	if err != nil {
		return err
	}

	backupRequest.BackedUpItems = map[itemKey]struct{}{}

	podVolumeTimeout := kb.resticTimeout
	if val := backupRequest.Annotations[api.PodVolumeOperationTimeoutAnnotation]; val != "" {
		parsed, err := time.ParseDuration(val)
		if err != nil {
			log.WithError(errors.WithStack(err)).Errorf("Unable to parse pod volume timeout annotation %s, using server value.", val)
		} else {
			podVolumeTimeout = parsed
		}
	}

	ctx, cancelFunc := context.WithTimeout(context.Background(), podVolumeTimeout)
	defer cancelFunc()

	var resticBackupper restic.Backupper
	if kb.resticBackupperFactory != nil {
		resticBackupper, err = kb.resticBackupperFactory.NewBackupper(ctx, backupRequest.Backup)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	gb := kb.groupBackupperFactory.newGroupBackupper(
		log,
		backupRequest,
		kb.dynamicFactory,
		kb.discoveryHelper,
		cohabitatingResources(),
		kb.podCommandExecutor,
		tw,
		resticBackupper,
		newPVCSnapshotTracker(),
		volumeSnapshotterGetter,
	)

	for _, group := range kb.discoveryHelper.Resources() {
		if err := gb.backupGroup(group); err != nil {
			log.WithError(err).WithField("apiGroup", group.String()).Error("Error backing up API group")
		}
	}

	return nil
}

func (kb *kubernetesBackupper) writeBackupVersion(tw *tar.Writer) error {
	versionFile := filepath.Join(api.MetadataDir, "version")
	versionString := fmt.Sprintf("%d\n", BackupVersion)

	hdr := &tar.Header{
		Name:     versionFile,
		Size:     int64(len(versionString)),
		Typeflag: tar.TypeReg,
		Mode:     0755,
		ModTime:  time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return errors.WithStack(err)
	}
	if _, err := tw.Write([]byte(versionString)); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

type tarWriter interface {
	io.Closer
	Write([]byte) (int, error)
	WriteHeader(*tar.Header) error
}
