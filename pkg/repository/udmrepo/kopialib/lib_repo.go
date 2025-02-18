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

package kopialib

import (
	"context"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/kopia/kopia/repo"
	"github.com/kopia/kopia/repo/compression"
	"github.com/kopia/kopia/repo/content/index"
	"github.com/kopia/kopia/repo/maintenance"
	"github.com/kopia/kopia/repo/manifest"
	"github.com/kopia/kopia/repo/object"
	"github.com/kopia/kopia/snapshot/snapshotmaintenance"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/vmware-tanzu/velero/pkg/kopia"
	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo"
)

type kopiaRepoService struct {
	logger logrus.FieldLogger
}

type kopiaRepository struct {
	rawRepo     repo.Repository
	rawWriter   repo.RepositoryWriter
	description string
	uploaded    int64
	openTime    time.Time
	throttle    logThrottle
	logger      logrus.FieldLogger
}

type kopiaMaintenance struct {
	mode      maintenance.Mode
	startTime time.Time
	uploaded  int64
	throttle  logThrottle
	logger    logrus.FieldLogger
}

type logThrottle struct {
	lastTime int64
	interval time.Duration
}

type kopiaObjectReader struct {
	rawReader object.Reader
}

type kopiaObjectWriter struct {
	rawWriter object.Writer
}

type openOptions struct{}

const (
	defaultLogInterval             = time.Second * 10
	defaultMaintainCheckPeriod     = time.Hour
	overwriteFullMaintainInterval  = time.Duration(0)
	overwriteQuickMaintainInterval = time.Duration(0)
)

var kopiaRepoOpen = repo.Open

// NewKopiaRepoService creates an instance of BackupRepoService implemented by Kopia
func NewKopiaRepoService(logger logrus.FieldLogger) udmrepo.BackupRepoService {
	ks := &kopiaRepoService{
		logger: logger,
	}

	return ks
}

func (ks *kopiaRepoService) Init(ctx context.Context, repoOption udmrepo.RepoOptions, createNew bool) error {
	repoCtx := kopia.SetupKopiaLog(ctx, ks.logger)

	if createNew {
		if err := CreateBackupRepo(repoCtx, repoOption); err != nil {
			return err
		}

		return writeInitParameters(repoCtx, repoOption, ks.logger)
	}
	return ConnectBackupRepo(repoCtx, repoOption)
}

func (ks *kopiaRepoService) Open(ctx context.Context, repoOption udmrepo.RepoOptions) (udmrepo.BackupRepo, error) {
	repoConfig := repoOption.ConfigFilePath
	if repoConfig == "" {
		return nil, errors.New("invalid config file path")
	}

	if _, err := os.Stat(repoConfig); os.IsNotExist(err) {
		return nil, errors.Wrapf(err, "repo config %s doesn't exist", repoConfig)
	}

	repoCtx := kopia.SetupKopiaLog(ctx, ks.logger)

	r, err := openKopiaRepo(repoCtx, repoConfig, repoOption.RepoPassword, nil)
	if err != nil {
		return nil, err
	}

	kr := kopiaRepository{
		rawRepo:     r,
		openTime:    time.Now(),
		description: repoOption.Description,
		throttle: logThrottle{
			interval: defaultLogInterval,
		},
		logger: ks.logger,
	}

	_, kr.rawWriter, err = r.NewWriter(repoCtx, repo.WriteSessionOptions{
		Purpose:  repoOption.Description,
		OnUpload: kr.updateProgress,
	})

	if err != nil {
		if e := r.Close(repoCtx); e != nil {
			ks.logger.WithError(e).Error("Failed to close raw repository on error")
		}

		return nil, errors.Wrap(err, "error to create repo writer")
	}

	return &kr, nil
}

func (ks *kopiaRepoService) Maintain(ctx context.Context, repoOption udmrepo.RepoOptions) error {
	repoConfig := repoOption.ConfigFilePath
	if repoConfig == "" {
		return errors.New("invalid config file path")
	}

	if _, err := os.Stat(repoConfig); os.IsNotExist(err) {
		return errors.Wrapf(err, "repo config %s doesn't exist", repoConfig)
	}

	repoCtx := kopia.SetupKopiaLog(ctx, ks.logger)

	ks.logger.Info("Start to open repo for maintenance, allow index write on load")

	r, err := openKopiaRepo(repoCtx, repoConfig, repoOption.RepoPassword, nil)
	if err != nil {
		return err
	}

	ks.logger.Info("Succeeded to open repo for maintenance")

	defer func() {
		c := r.Close(repoCtx)
		if c != nil {
			ks.logger.WithError(c).Error("Failed to close repo")
		}
	}()

	km := kopiaMaintenance{
		mode:      maintenance.ModeAuto,
		startTime: time.Now(),
		throttle: logThrottle{
			interval: defaultLogInterval,
		},
		logger: ks.logger,
	}

	if mode, exist := repoOption.GeneralOptions[udmrepo.GenOptionMaintainMode]; exist {
		if strings.EqualFold(mode, udmrepo.GenOptionMaintainFull) {
			km.mode = maintenance.ModeFull
		} else if strings.EqualFold(mode, udmrepo.GenOptionMaintainQuick) {
			km.mode = maintenance.ModeQuick
		}
	}

	err = repo.DirectWriteSession(repoCtx, r.(repo.DirectRepository), repo.WriteSessionOptions{
		Purpose:  "UdmRepoMaintenance",
		OnUpload: km.maintainProgress,
	}, func(ctx context.Context, dw repo.DirectRepositoryWriter) error {
		return km.runMaintenance(ctx, dw)
	})

	if err != nil {
		return errors.Wrap(err, "error to maintain repo")
	}

	return nil
}

func (ks *kopiaRepoService) DefaultMaintenanceFrequency() time.Duration {
	return defaultMaintainCheckPeriod
}

func (km *kopiaMaintenance) runMaintenance(ctx context.Context, rep repo.DirectRepositoryWriter) error {
	err := snapshotmaintenance.Run(kopia.SetupKopiaLog(ctx, km.logger), rep, km.mode, false, maintenance.SafetyFull)
	if err != nil {
		return errors.Wrapf(err, "error to run maintenance under mode %s", km.mode)
	}

	return nil
}

// maintainProgress is called when the repository writes a piece of blob data to the storage during the maintenance
func (km *kopiaMaintenance) maintainProgress(uploaded int64) {
	total := atomic.AddInt64(&km.uploaded, uploaded)

	if km.throttle.shouldLog() {
		km.logger.WithFields(
			logrus.Fields{
				"Start Time": km.startTime.Format(time.RFC3339Nano),
				"Current":    time.Now().Format(time.RFC3339Nano),
			},
		).Debugf("Repo maintenance uploaded %d bytes.", total)
	}
}

func (kr *kopiaRepository) OpenObject(ctx context.Context, id udmrepo.ID) (udmrepo.ObjectReader, error) {
	if kr.rawRepo == nil {
		return nil, errors.New("repo is closed or not open")
	}

	objID, err := object.ParseID(string(id))
	if err != nil {
		return nil, errors.Wrapf(err, "error to parse object ID from %v", id)
	}

	reader, err := kr.rawRepo.OpenObject(kopia.SetupKopiaLog(ctx, kr.logger), objID)
	if err != nil {
		return nil, errors.Wrap(err, "error to open object")
	}

	return &kopiaObjectReader{
		rawReader: reader,
	}, nil
}

func (kr *kopiaRepository) GetManifest(ctx context.Context, id udmrepo.ID, mani *udmrepo.RepoManifest) error {
	if kr.rawRepo == nil {
		return errors.New("repo is closed or not open")
	}

	metadata, err := kr.rawRepo.GetManifest(kopia.SetupKopiaLog(ctx, kr.logger), manifest.ID(id), mani.Payload)
	if err != nil {
		return errors.Wrap(err, "error to get manifest")
	}

	mani.Metadata = getManifestEntryFromKopia(metadata)

	return nil
}

func (kr *kopiaRepository) FindManifests(ctx context.Context, filter udmrepo.ManifestFilter) ([]*udmrepo.ManifestEntryMetadata, error) {
	if kr.rawRepo == nil {
		return nil, errors.New("repo is closed or not open")
	}

	metadata, err := kr.rawRepo.FindManifests(kopia.SetupKopiaLog(ctx, kr.logger), filter.Labels)
	if err != nil {
		return nil, errors.Wrap(err, "error to find manifests")
	}

	return getManifestEntriesFromKopia(metadata), nil
}

func (kr *kopiaRepository) Time() time.Time {
	if kr.rawRepo == nil {
		return time.Time{}
	}

	return kr.rawRepo.Time()
}

func (kr *kopiaRepository) Close(ctx context.Context) error {
	if kr.rawWriter != nil {
		err := kr.rawWriter.Close(kopia.SetupKopiaLog(ctx, kr.logger))
		if err != nil {
			return errors.Wrap(err, "error to close repo writer")
		}

		kr.rawWriter = nil
	}

	if kr.rawRepo != nil {
		err := kr.rawRepo.Close(kopia.SetupKopiaLog(ctx, kr.logger))
		if err != nil {
			return errors.Wrap(err, "error to close repo")
		}

		kr.rawRepo = nil
	}

	return nil
}

func (kr *kopiaRepository) NewObjectWriter(ctx context.Context, opt udmrepo.ObjectWriteOptions) udmrepo.ObjectWriter {
	if kr.rawWriter == nil {
		return nil
	}

	writer := kr.rawWriter.NewObjectWriter(kopia.SetupKopiaLog(ctx, kr.logger), object.WriterOptions{
		Description:        opt.Description,
		Prefix:             index.IDPrefix(opt.Prefix),
		AsyncWrites:        opt.AsyncWrites,
		Compressor:         getCompressorForObject(opt),
		MetadataCompressor: getMetadataCompressor(),
	})

	if writer == nil {
		return nil
	}

	return &kopiaObjectWriter{
		rawWriter: writer,
	}
}

func (kr *kopiaRepository) PutManifest(ctx context.Context, manifest udmrepo.RepoManifest) (udmrepo.ID, error) {
	if kr.rawWriter == nil {
		return "", errors.New("repo writer is closed or not open")
	}

	id, err := kr.rawWriter.PutManifest(kopia.SetupKopiaLog(ctx, kr.logger), manifest.Metadata.Labels, manifest.Payload)
	if err != nil {
		return "", errors.Wrap(err, "error to put manifest")
	}

	return udmrepo.ID(id), nil
}

func (kr *kopiaRepository) DeleteManifest(ctx context.Context, id udmrepo.ID) error {
	if kr.rawWriter == nil {
		return errors.New("repo writer is closed or not open")
	}

	err := kr.rawWriter.DeleteManifest(kopia.SetupKopiaLog(ctx, kr.logger), manifest.ID(id))
	if err != nil {
		return errors.Wrap(err, "error to delete manifest")
	}

	return nil
}

func (kr *kopiaRepository) Flush(ctx context.Context) error {
	if kr.rawWriter == nil {
		return errors.New("repo writer is closed or not open")
	}

	err := kr.rawWriter.Flush(kopia.SetupKopiaLog(ctx, kr.logger))
	if err != nil {
		return errors.Wrap(err, "error to flush repo")
	}

	return nil
}

func (kr *kopiaRepository) GetAdvancedFeatures() udmrepo.AdvancedFeatureInfo {
	return udmrepo.AdvancedFeatureInfo{
		MultiPartBackup: true,
	}
}

func (kr *kopiaRepository) ConcatenateObjects(ctx context.Context, objectIDs []udmrepo.ID) (udmrepo.ID, error) {
	if kr.rawWriter == nil {
		return "", errors.New("repo writer is closed or not open")
	}

	if len(objectIDs) == 0 {
		return udmrepo.ID(""), errors.New("object list is empty")
	}

	rawIDs := []object.ID{}
	for _, id := range objectIDs {
		rawID, err := object.ParseID(string(id))
		if err != nil {
			return udmrepo.ID(""), errors.Wrapf(err, "error to parse object ID from %v", id)
		}

		rawIDs = append(rawIDs, rawID)
	}

	result, err := kr.rawWriter.ConcatenateObjects(ctx, rawIDs, repo.ConcatenateOptions{
		Compressor: getMetadataCompressor(),
	})
	if err != nil {
		return udmrepo.ID(""), errors.Wrap(err, "error to concatenate objects")
	}

	return udmrepo.ID(result.String()), nil
}

// updateProgress is called when the repository writes a piece of blob data to the storage during data write
func (kr *kopiaRepository) updateProgress(uploaded int64) {
	total := atomic.AddInt64(&kr.uploaded, uploaded)

	if kr.throttle.shouldLog() {
		kr.logger.WithFields(
			logrus.Fields{
				"Description": kr.description,
				"Open Time":   kr.openTime.Format(time.RFC3339Nano),
				"Current":     time.Now().Format(time.RFC3339Nano),
			},
		).Debugf("Repo uploaded %d bytes.", total)
	}
}

func (kor *kopiaObjectReader) Read(p []byte) (int, error) {
	if kor.rawReader == nil {
		return 0, errors.New("object reader is closed or not open")
	}

	return kor.rawReader.Read(p)
}

func (kor *kopiaObjectReader) Seek(offset int64, whence int) (int64, error) {
	if kor.rawReader == nil {
		return -1, errors.New("object reader is closed or not open")
	}

	return kor.rawReader.Seek(offset, whence)
}

func (kor *kopiaObjectReader) Close() error {
	if kor.rawReader == nil {
		return nil
	}

	err := kor.rawReader.Close()
	if err != nil {
		return err
	}

	kor.rawReader = nil

	return nil
}

func (kor *kopiaObjectReader) Length() int64 {
	if kor.rawReader == nil {
		return -1
	}

	return kor.rawReader.Length()
}

func (kow *kopiaObjectWriter) Write(p []byte) (int, error) {
	if kow.rawWriter == nil {
		return 0, errors.New("object writer is closed or not open")
	}

	return kow.rawWriter.Write(p)
}

func (kow *kopiaObjectWriter) Seek(offset int64, whence int) (int64, error) {
	return -1, errors.New("not supported")
}

func (kow *kopiaObjectWriter) Checkpoint() (udmrepo.ID, error) {
	if kow.rawWriter == nil {
		return udmrepo.ID(""), errors.New("object writer is closed or not open")
	}

	id, err := kow.rawWriter.Checkpoint()
	if err != nil {
		return udmrepo.ID(""), errors.Wrap(err, "error to checkpoint object")
	}

	return udmrepo.ID(id.String()), nil
}

func (kow *kopiaObjectWriter) Result() (udmrepo.ID, error) {
	if kow.rawWriter == nil {
		return udmrepo.ID(""), errors.New("object writer is closed or not open")
	}

	id, err := kow.rawWriter.Result()
	if err != nil {
		return udmrepo.ID(""), errors.Wrap(err, "error to wait object")
	}

	return udmrepo.ID(id.String()), nil
}

func (kow *kopiaObjectWriter) Close() error {
	if kow.rawWriter == nil {
		return nil
	}

	err := kow.rawWriter.Close()
	if err != nil {
		return err
	}

	kow.rawWriter = nil

	return nil
}

// getCompressorForObject returns the compressor for an object, at present, we don't support compression
func getCompressorForObject(_ udmrepo.ObjectWriteOptions) compression.Name {
	return ""
}

// getMetadataCompressor returns the compressor for metadata, return kopia's default since we don't support compression
func getMetadataCompressor() compression.Name {
	return "zstd-fastest"
}

func getManifestEntryFromKopia(mani *manifest.EntryMetadata) *udmrepo.ManifestEntryMetadata {
	return &udmrepo.ManifestEntryMetadata{
		ID:      udmrepo.ID(mani.ID),
		Labels:  mani.Labels,
		Length:  int32(mani.Length),
		ModTime: mani.ModTime,
	}
}

func getManifestEntriesFromKopia(mani []*manifest.EntryMetadata) []*udmrepo.ManifestEntryMetadata {
	var ret []*udmrepo.ManifestEntryMetadata

	for _, entry := range mani {
		ret = append(ret, &udmrepo.ManifestEntryMetadata{
			ID:      udmrepo.ID(entry.ID),
			Labels:  entry.Labels,
			Length:  int32(entry.Length),
			ModTime: entry.ModTime,
		})
	}

	return ret
}

func (lt *logThrottle) shouldLog() bool {
	nextOutputTime := atomic.LoadInt64(&lt.lastTime)
	if nowNano := time.Now().UnixNano(); nowNano > nextOutputTime {
		if atomic.CompareAndSwapInt64(&lt.lastTime, nextOutputTime, nowNano+lt.interval.Nanoseconds()) {
			return true
		}
	}

	return false
}

func openKopiaRepo(ctx context.Context, configFile string, password string, _ *openOptions) (repo.Repository, error) {
	r, err := kopiaRepoOpen(ctx, configFile, password, &repo.Options{})
	if os.IsNotExist(err) {
		return nil, errors.Wrap(err, "error to open repo, repo doesn't exist")
	}

	if err != nil {
		return nil, errors.Wrap(err, "error to open repo")
	}

	return r, nil
}

func writeInitParameters(ctx context.Context, repoOption udmrepo.RepoOptions, logger logrus.FieldLogger) error {
	r, err := openKopiaRepo(ctx, repoOption.ConfigFilePath, repoOption.RepoPassword, nil)
	if err != nil {
		return err
	}

	defer func() {
		c := r.Close(ctx)
		if c != nil {
			logger.WithError(c).Error("Failed to close repo")
		}
	}()

	err = repo.WriteSession(ctx, r, repo.WriteSessionOptions{
		Purpose: "set init parameters",
	}, func(ctx context.Context, w repo.RepositoryWriter) error {
		p := maintenance.DefaultParams()

		if overwriteFullMaintainInterval != time.Duration(0) {
			logger.Infof("Full maintenance interval change from %v to %v", p.FullCycle.Interval, overwriteFullMaintainInterval)
			p.FullCycle.Interval = overwriteFullMaintainInterval
		}

		if overwriteQuickMaintainInterval != time.Duration(0) {
			logger.Infof("Quick maintenance interval change from %v to %v", p.QuickCycle.Interval, overwriteQuickMaintainInterval)
			p.QuickCycle.Interval = overwriteQuickMaintainInterval
		}
		// the repoOption.StorageOptions are set via
		// udmrepo.WithStoreOptions -> udmrepo.GetStoreOptions (interface)
		// -> pkg/repository/provider.GetStoreOptions(param interface{}) -> pkg/repository/provider.getStorageVariables(..., backupRepoConfig)
		// where backupRepoConfig comes from param.(RepoParam).BackupRepo.Spec.RepositoryConfig map[string]string
		// where RepositoryConfig comes from pkg/controller/getBackupRepositoryConfig(...)
		// where it gets a configMap name from pkg/cmd/server/config/Config.BackupRepoConfig
		// which gets set via velero server flag `backup-repository-configmap` "The name of ConfigMap containing backup repository configurations."
		// and data stored as json under ConfigMap.Data[repoType] where repoType is BackupRepository.Spec.RepositoryType: either kopia or restic
		// repoOption.StorageOptions[udmrepo.StoreOptionKeyFullMaintenanceInterval] would for example look like
		// configMapName.data.kopia: {"fullMaintenanceInterval": "eagerGC"}
		fullMaintIntervalOption := udmrepo.FullMaintenanceIntervalOptions(repoOption.StorageOptions[udmrepo.StoreOptionKeyFullMaintenanceInterval])
		priorMaintInterval := p.FullCycle.Interval
		switch fullMaintIntervalOption {
		case udmrepo.FastGC:
			p.FullCycle.Interval = udmrepo.FastGCInterval
		case udmrepo.EagerGC:
			p.FullCycle.Interval = udmrepo.EagerGCInterval
		case udmrepo.NormalGC:
			p.FullCycle.Interval = udmrepo.NormalGCInterval
		default:
			return errors.Errorf("invalid full maintenance interval option %s", fullMaintIntervalOption)
		}
		logger.Infof("Full maintenance interval change from %v to %v", priorMaintInterval, p.FullCycle.Interval)

		p.Owner = r.ClientOptions().UsernameAtHost()

		if err := maintenance.SetParams(ctx, w, &p); err != nil {
			return errors.Wrap(err, "error to set maintenance params")
		}

		return nil
	})

	if err != nil {
		return errors.Wrap(err, "error to init write repo parameters")
	}

	return nil
}
