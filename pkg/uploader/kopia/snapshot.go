/*
Copyright The Velero Contributors.

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

package kopia

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo"
	"github.com/vmware-tanzu/velero/pkg/uploader"

	"github.com/kopia/kopia/fs"
	"github.com/kopia/kopia/fs/localfs"
	"github.com/kopia/kopia/repo"
	"github.com/kopia/kopia/repo/manifest"
	"github.com/kopia/kopia/snapshot"
	"github.com/kopia/kopia/snapshot/policy"
	"github.com/kopia/kopia/snapshot/restore"
	"github.com/kopia/kopia/snapshot/snapshotfs"
	"github.com/pkg/errors"
)

//All function mainly used to make testing more convenient
var treeForSourceFunc = policy.TreeForSource
var applyRetentionPolicyFunc = policy.ApplyRetentionPolicy
var setPolicyFunc = policy.SetPolicy
var saveSnapshotFunc = snapshot.SaveSnapshot
var loadSnapshotFunc = snapshot.LoadSnapshot

//SnapshotUploader which mainly used for UT test that could overwrite Upload interface
type SnapshotUploader interface {
	Upload(
		ctx context.Context,
		source fs.Entry,
		policyTree *policy.Tree,
		sourceInfo snapshot.SourceInfo,
		previousManifests ...*snapshot.Manifest,
	) (*snapshot.Manifest, error)
}

func newOptionalInt(b policy.OptionalInt) *policy.OptionalInt {
	return &b
}

//setupDefaultPolicy set default policy for kopia
func setupDefaultPolicy(ctx context.Context, rep repo.RepositoryWriter, sourceInfo snapshot.SourceInfo) error {
	return setPolicyFunc(ctx, rep, sourceInfo, &policy.Policy{
		RetentionPolicy: policy.RetentionPolicy{
			KeepLatest: newOptionalInt(math.MaxInt32),
		},
		CompressionPolicy: policy.CompressionPolicy{
			CompressorName: "none",
		},
		UploadPolicy: policy.UploadPolicy{
			MaxParallelFileReads: newOptionalInt(policy.OptionalInt(runtime.NumCPU())),
		},
		SchedulingPolicy: policy.SchedulingPolicy{
			Manual: true,
		},
	})
}

//Backup backup specific sourcePath and update progress
func Backup(ctx context.Context, fsUploader *snapshotfs.Uploader, repoWriter repo.RepositoryWriter, sourcePath string,
	parentSnapshot string, log logrus.FieldLogger) (*uploader.SnapshotInfo, error) {
	if fsUploader == nil {
		return nil, errors.New("get empty kopia uploader")
	}
	dir, err := filepath.Abs(sourcePath)
	if err != nil {
		return nil, errors.Wrapf(err, "Invalid source path '%s'", sourcePath)
	}

	sourceInfo := snapshot.SourceInfo{
		UserName: udmrepo.GetRepoUser(),
		Host:     udmrepo.GetRepoDomain(),
		Path:     filepath.Clean(dir),
	}

	rootDir, err := getLocalFSEntry(sourceInfo.Path)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to get local filesystem entry")
	}
	snapID, snapshotSize, err := SnapshotSource(ctx, repoWriter, fsUploader, sourceInfo, rootDir, parentSnapshot, log, "Kopia Uploader")
	if err != nil {
		return nil, err
	}

	snapshotInfo := &uploader.SnapshotInfo{
		ID:   snapID,
		Size: snapshotSize,
	}

	return snapshotInfo, nil
}

func getLocalFSEntry(path0 string) (fs.Entry, error) {
	path, err := resolveSymlink(path0)
	if err != nil {
		return nil, errors.Wrap(err, "resolveSymlink")
	}

	e, err := localfs.NewEntry(path)
	if err != nil {
		return nil, errors.Wrap(err, "can't get local fs entry")
	}

	return e, nil
}

//resolveSymlink returns the path name after the evaluation of any symbolic links
func resolveSymlink(path string) (string, error) {
	st, err := os.Lstat(path)
	if err != nil {
		return "", errors.Wrap(err, "stat")
	}

	if (st.Mode() & os.ModeSymlink) == 0 {
		return path, nil
	}

	return filepath.EvalSymlinks(path)
}

//SnapshotSource which setup policy for snapshot, upload snapshot, update progress
func SnapshotSource(
	ctx context.Context,
	rep repo.RepositoryWriter,
	u SnapshotUploader,
	sourceInfo snapshot.SourceInfo,
	rootDir fs.Entry,
	parentSnapshot string,
	log logrus.FieldLogger,
	description string,
) (string, int64, error) {
	log.Info("Start to snapshot...")
	snapshotStartTime := time.Now()

	var previous []*snapshot.Manifest
	if parentSnapshot != "" {
		mani, err := loadSnapshotFunc(ctx, rep, manifest.ID(parentSnapshot))
		if err != nil {
			return "", 0, errors.Wrapf(err, "Failed to load previous snapshot %v from kopia", parentSnapshot)
		}

		previous = append(previous, mani)
	} else {
		pre, err := findPreviousSnapshotManifest(ctx, rep, sourceInfo, nil)
		if err != nil {
			return "", 0, errors.Wrapf(err, "Failed to find previous kopia snapshot manifests for si %v", sourceInfo)
		}

		previous = pre
	}
	var manifest *snapshot.Manifest
	if err := setupDefaultPolicy(ctx, rep, sourceInfo); err != nil {
		return "", 0, errors.Wrapf(err, "unable to set policy for si %v", sourceInfo)
	}

	policyTree, err := treeForSourceFunc(ctx, rep, sourceInfo)
	if err != nil {
		return "", 0, errors.Wrapf(err, "unable to create policy getter for si %v", sourceInfo)
	}

	manifest, err = u.Upload(ctx, rootDir, policyTree, sourceInfo, previous...)
	if err != nil {
		return "", 0, errors.Wrapf(err, "Failed to upload the kopia snapshot for si %v", sourceInfo)
	}

	manifest.Description = description

	if _, err = saveSnapshotFunc(ctx, rep, manifest); err != nil {
		return "", 0, errors.Wrapf(err, "Failed to save kopia manifest %v", manifest.ID)
	}
	_, err = applyRetentionPolicyFunc(ctx, rep, sourceInfo, true)
	if err != nil {
		return "", 0, errors.Wrapf(err, "Failed to apply kopia retention policy for si %v", sourceInfo)
	}
	if err = rep.Flush(ctx); err != nil {
		return "", 0, errors.Wrapf(err, "Failed to flush kopia repository")
	}
	log.Infof("Created snapshot with root %v and ID %v in %v", manifest.RootObjectID(), manifest.ID, time.Since(snapshotStartTime).Truncate(time.Second))
	return reportSnapshotStatus(manifest)
}

func reportSnapshotStatus(manifest *snapshot.Manifest) (string, int64, error) {
	manifestID := manifest.ID
	snapSize := manifest.Stats.TotalFileSize

	var errs []string
	if ds := manifest.RootEntry.DirSummary; ds != nil {
		for _, ent := range ds.FailedEntries {
			errs = append(errs, ent.Error)
		}
	}
	if len(errs) != 0 {
		return "", 0, errors.New(strings.Join(errs, "\n"))
	}

	return string(manifestID), snapSize, nil
}

// findPreviousSnapshotManifest returns the list of previous snapshots for a given source, including
// last complete snapshot following it.
func findPreviousSnapshotManifest(ctx context.Context, rep repo.Repository, sourceInfo snapshot.SourceInfo, noLaterThan *time.Time) ([]*snapshot.Manifest, error) {
	man, err := snapshot.ListSnapshots(ctx, rep, sourceInfo)
	if err != nil {
		return nil, err
	}

	var previousComplete *snapshot.Manifest
	var result []*snapshot.Manifest

	for _, p := range man {
		if noLaterThan != nil && p.StartTime.After(*noLaterThan) {
			continue
		}

		if p.IncompleteReason == "" && (previousComplete == nil || p.StartTime.After(previousComplete.StartTime)) {
			previousComplete = p
		}
	}

	if previousComplete != nil {
		result = append(result, previousComplete)
	}

	return result, nil
}

//Restore restore specific sourcePath with given snapshotID and update progress
func Restore(ctx context.Context, rep repo.RepositoryWriter, progress *KopiaProgress, snapshotID, dest string, log logrus.FieldLogger, cancleCh chan struct{}) (int64, int32, error) {
	log.Info("Start to restore...")

	rootEntry, err := snapshotfs.FilesystemEntryFromIDWithPath(ctx, rep, snapshotID, false)
	if err != nil {
		return 0, 0, errors.Wrapf(err, "Unable to get filesystem entry for snapshot %v", snapshotID)
	}

	path, err := filepath.Abs(dest)
	if err != nil {
		return 0, 0, errors.Wrapf(err, "Unable to resolve path %v", dest)
	}

	output := &restore.FilesystemOutput{
		TargetPath:             path,
		OverwriteDirectories:   true,
		OverwriteFiles:         true,
		OverwriteSymlinks:      true,
		IgnorePermissionErrors: true,
	}

	stat, err := restore.Entry(ctx, rep, output, rootEntry, restore.Options{
		Parallel:               runtime.NumCPU(),
		RestoreDirEntryAtDepth: math.MaxInt32,
		Cancel:                 cancleCh,
		ProgressCallback: func(ctx context.Context, stats restore.Stats) {
			progress.ProgressBytes(stats.RestoredTotalFileSize, stats.EnqueuedTotalFileSize)
		},
	})

	if err != nil {
		return 0, 0, errors.Wrapf(err, "Failed to copy snapshot data to the target")
	}
	return stat.RestoredTotalFileSize, stat.RestoredFileCount, nil
}
