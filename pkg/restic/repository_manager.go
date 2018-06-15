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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

	// ChangeKey changes the encryption key for a repo.
	ChangeKey(name string) error

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

func (rm *repositoryManager) ChangeKey(name string) error {
	log := rm.log.WithField("repo", name)
	log.Debug("Changing key")

	repo, err := getReadyRepo(rm.repoLister, rm.namespace, name)
	if err != nil {
		return err
	}

	rm.repoLocker.LockExclusive(name)
	defer rm.repoLocker.UnlockExclusive(name)

	secret, err := rm.secretsLister.Secrets(name).Get(CredentialsSecretName)
	if err != nil {
		return errors.WithStack(err)
	}

	// if there's data at the NewCredentialsKey within the secret, it could mean any of the following:
	//	- this is our first time processing this key-change request
	//	- we attempted to change the repo key already but got an error
	//  - we successfully changed the repo key, but failed to update the secret to
	//	  reflect the change
	if newKey, ok := secret.Data[NewCredentialsKey]; ok {
		log.Debugf("Found key in secret at %s", NewCredentialsKey)

		if string(newKey) == string(secret.Data[CurrentCredentialsKey]) {
			log.Debugf("New key is the same as current key")

			return patchSecret(secret, func(s *corev1api.Secret) {
				delete(s.Data, NewCredentialsKey)
			}, rm.secretsClient)
		}

		newKeyFile, err := writeTempFile("", "", newKey)
		if err != nil {
			return err
		}
		defer os.Remove(newKeyFile)

		// if it's not valid, we need to run the restic cmd to change repo keys
		if !isKeyValid(repo.Spec.ResticIdentifier, newKeyFile) {
			log.Debugf("Key in %s is not valid", NewCredentialsKey)
			if err := rm.exec(ChangeKeyCommand(repo.Spec.ResticIdentifier, newKeyFile)); err != nil {
				return err
			}
			log.Debugf("Ran restic key passwd command successfully")
		}

		// the new key is now valid, so patch the secret to move keys around appropriately
		if err := patchSecret(secret, func(s *corev1api.Secret) {
			s.Data[OldCredentialsKey], s.Data[CurrentCredentialsKey] = s.Data[CurrentCredentialsKey], s.Data[NewCredentialsKey]
			delete(s.Data, NewCredentialsKey)
		}, rm.secretsClient); err != nil {
			return err
		}
		log.Debugf("Patched secret to move %s into %s and %s into %s", NewCredentialsKey, CurrentCredentialsKey, CurrentCredentialsKey, OldCredentialsKey)
	}

	// if there's data at the OldCredentialsKey within the secret, it could mean any of the following:
	//	- this is our first time processing this key-change request
	//  - a prior key-change request partially succeeded, meaning the new key was added to the repo
	//    but the old key was not removed
	//	- we successfully changed the repo key, but failed to remove the old key from the secret
	if oldKey, ok := secret.Data[OldCredentialsKey]; ok {
		log.Debugf("Found key in secret at %s", OldCredentialsKey)
		oldKeyFile, err := writeTempFile("", "", oldKey)
		if err != nil {
			return err
		}
		defer os.Remove(oldKeyFile)

		if isKeyValid(repo.Spec.ResticIdentifier, oldKeyFile) {
			log.Debugf("Key in %s is valid", OldCredentialsKey)
			id, err := getKeyID(repo.Spec.ResticIdentifier, oldKeyFile)
			if err != nil {
				return err
			}
			log.Debugf("Key in %s has restic id %s", OldCredentialsKey, id)
			if err := rm.exec(RemoveKeyCommand(repo.Spec.ResticIdentifier, id)); err != nil {
				return err
			}
			log.Debugf("Ran restic key remove command successfully")
		}

		if err := patchSecret(secret, func(s *corev1api.Secret) {
			delete(s.Data, OldCredentialsKey)
		}, rm.secretsClient); err != nil {
			return err
		}
		log.Debugf("Patched secret to remove %s", OldCredentialsKey)
	}

	return nil
}

func getKeyID(resticIdentifer string, keyFile string) (string, error) {
	cmd := ListKeysCommand(resticIdentifer)
	cmd.PasswordFile = keyFile

	stdout, _, err := arkexec.RunCommand(cmd.Cmd())
	if err != nil {
		return "", err
	}

	// TODO(1.0) once https://github.com/restic/restic/pull/1853 makes it into a restic
	// release, we can replace this table-parsing with reading JSON.
	for _, line := range strings.Split(stdout, "\n") {
		// the current key is prefixed with a '*'
		if !strings.HasPrefix(line, "*") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) <= 0 {
			return "", errors.New("unable to get current key's ID")
		}

		id := strings.TrimPrefix(fields[0], "*")
		if len(id) <= 0 {
			return "", errors.New("unable to get current key's ID")
		}

		return id, nil
	}

	return "", errors.New("unable to find current key")
}

func isKeyValid(resticIdentifier string, keyFile string) bool {
	// use `restic key list` as a test cmd to see if we can connect or not
	cmd := ListKeysCommand(resticIdentifier)
	cmd.PasswordFile = keyFile

	// no error means key is valid
	return cmd.Cmd().Run() == nil
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

func patchSecret(secret *corev1api.Secret, mutate func(s *corev1api.Secret), secretClient corev1client.SecretsGetter) error {
	original, err := json.Marshal(secret)
	if err != nil {
		return errors.Wrapf(err, "error marshalling secret to JSON")
	}

	mutate(secret)

	updated, err := json.Marshal(secret)
	if err != nil {
		return errors.Wrapf(err, "error marshalling updated secret to JSON")
	}

	patch, err := jsonpatch.CreateMergePatch(original, updated)
	if err != nil {
		return errors.Wrapf(err, "error creating merge patch")
	}

	if secret, err = secretClient.Secrets(secret.Namespace).Patch(secret.Name, types.MergePatchType, patch); err != nil {
		return errors.Wrap(err, "error patching secret")
	}

	return nil
}
