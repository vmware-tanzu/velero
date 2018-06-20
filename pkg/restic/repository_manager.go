/*
Copyright 2018 the Heptio Ark contributors.

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

package restic

import (
	"context"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	arkv1api "github.com/heptio/ark/pkg/apis/ark/v1"
	clientset "github.com/heptio/ark/pkg/generated/clientset/versioned"
	arkv1informers "github.com/heptio/ark/pkg/generated/informers/externalversions/ark/v1"
	arkv1listers "github.com/heptio/ark/pkg/generated/listers/ark/v1"
	arkexec "github.com/heptio/ark/pkg/util/exec"
)

// RepositoryManager executes commands against restic repositories.
type RepositoryManager interface {
	// InitRepo initializes a repo with the specified name.
	InitRepo(name string) error

	// CheckRepo checks the specified repo for errors.
	CheckRepo(name string) error

	// PruneRepo deletes unused data from a repo.
	PruneRepo(name string) error

	// Forget removes a snapshot from the list of
	// available snapshots in a repo.
	Forget(snapshot SnapshotIdentifier) error

	BackupperFactory

	RestorerFactory
}

// BackupperFactory can construct restic backuppers.
type BackupperFactory interface {
	// NewBackupper returns a restic backupper for use during a single
	// Ark backup.
	NewBackupper(context.Context, *arkv1api.Backup) (Backupper, error)
}

// RestorerFactory can construct restic restorers.
type RestorerFactory interface {
	// NewRestorer returns a restic restorer for use during a single
	// Ark restore.
	NewRestorer(context.Context, *arkv1api.Restore) (Restorer, error)
}

type repositoryManager struct {
	namespace          string
	arkClient          clientset.Interface
	secretsLister      corev1listers.SecretLister
	secretsClient      corev1client.SecretsGetter
	repoLister         arkv1listers.ResticRepositoryLister
	repoInformerSynced cache.InformerSynced
	log                logrus.FieldLogger
	repoLocker         *repoLocker
}

// NewRepositoryManager constructs a RepositoryManager.
func NewRepositoryManager(
	ctx context.Context,
	namespace string,
	arkClient clientset.Interface,
	secretsInformer cache.SharedIndexInformer,
	secretsClient corev1client.SecretsGetter,
	repoInformer arkv1informers.ResticRepositoryInformer,
	log logrus.FieldLogger,
) (RepositoryManager, error) {
	rm := &repositoryManager{
		namespace:          namespace,
		arkClient:          arkClient,
		secretsLister:      corev1listers.NewSecretLister(secretsInformer.GetIndexer()),
		secretsClient:      secretsClient,
		repoLister:         repoInformer.Lister(),
		repoInformerSynced: repoInformer.Informer().HasSynced,
		log:                log,
		repoLocker:         newRepoLocker(),
	}

	if !cache.WaitForCacheSync(ctx.Done(), secretsInformer.HasSynced) {
		return nil, errors.New("timed out waiting for cache to sync")
	}

	return rm, nil
}

func (rm *repositoryManager) NewBackupper(ctx context.Context, backup *arkv1api.Backup) (Backupper, error) {
	informer := arkv1informers.NewFilteredPodVolumeBackupInformer(
		rm.arkClient,
		backup.Namespace,
		0,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		func(opts *metav1.ListOptions) {
			opts.LabelSelector = fmt.Sprintf("%s=%s", arkv1api.BackupUIDLabel, backup.UID)
		},
	)

	b := newBackupper(ctx, rm, informer, rm.repoLister)

	go informer.Run(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced, rm.repoInformerSynced) {
		return nil, errors.New("timed out waiting for caches to sync")
	}

	return b, nil
}

func (rm *repositoryManager) NewRestorer(ctx context.Context, restore *arkv1api.Restore) (Restorer, error) {
	informer := arkv1informers.NewFilteredPodVolumeRestoreInformer(
		rm.arkClient,
		restore.Namespace,
		0,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		func(opts *metav1.ListOptions) {
			opts.LabelSelector = fmt.Sprintf("%s=%s", arkv1api.RestoreUIDLabel, restore.UID)
		},
	)

	r := newRestorer(ctx, rm, informer, rm.repoLister)

	go informer.Run(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return nil, errors.New("timed out waiting for cache to sync")
	}

	return r, nil
}

func (rm *repositoryManager) InitRepo(name string) error {
	repo, err := getRepo(rm.repoLister, rm.namespace, name)
	if err != nil {
		return err
	}

	rm.repoLocker.LockExclusive(name)
	defer rm.repoLocker.UnlockExclusive(name)

	return rm.exec(InitCommand(repo.Spec.ResticIdentifier))
}

func (rm *repositoryManager) CheckRepo(name string) error {
	repo, err := getRepo(rm.repoLister, rm.namespace, name)
	if err != nil {
		return err
	}

	rm.repoLocker.LockExclusive(name)
	defer rm.repoLocker.UnlockExclusive(name)

	cmd := CheckCommand(repo.Spec.ResticIdentifier)

	return rm.exec(cmd)
}

func (rm *repositoryManager) PruneRepo(name string) error {
	repo, err := getReadyRepo(rm.repoLister, rm.namespace, name)
	if err != nil {
		return err
	}

	rm.repoLocker.LockExclusive(name)
	defer rm.repoLocker.UnlockExclusive(name)

	cmd := PruneCommand(repo.Spec.ResticIdentifier)

	return rm.exec(cmd)
}

func (rm *repositoryManager) Forget(snapshot SnapshotIdentifier) error {
	repo, err := getReadyRepo(rm.repoLister, rm.namespace, snapshot.Repo)
	if err != nil {
		return err
	}

	rm.repoLocker.LockExclusive(snapshot.Repo)
	defer rm.repoLocker.UnlockExclusive(snapshot.Repo)

	cmd := ForgetCommand(repo.Spec.ResticIdentifier, snapshot.SnapshotID)

	return rm.exec(cmd)
}

func (rm *repositoryManager) exec(cmd *Command) error {
	file, err := TempCredentialsFile(rm.secretsLister, cmd.RepoName())
	if err != nil {
		return err
	}
	// ignore error since there's nothing we can do and it's a temp file.
	defer os.Remove(file)

	cmd.PasswordFile = file

	stdout, stderr, err := arkexec.RunCommand(cmd.Cmd())
	rm.log.WithFields(logrus.Fields{
		"repository": cmd.RepoName(),
		"command":    cmd.String(),
		"stdout":     stdout,
		"stderr":     stderr,
	}).Debugf("Ran restic command")
	if err != nil {
		return errors.Wrapf(err, "error running command=%s, stdout=%s, stderr=%s", cmd.String(), stdout, stderr)
	}

	return nil
}
