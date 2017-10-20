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

package server

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2/google"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	kcorev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/backup"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cloudprovider"
	arkaws "github.com/heptio/ark/pkg/cloudprovider/aws"
	"github.com/heptio/ark/pkg/cloudprovider/azure"
	"github.com/heptio/ark/pkg/cloudprovider/gcp"
	"github.com/heptio/ark/pkg/cmd"
	"github.com/heptio/ark/pkg/cmd/util/flag"
	"github.com/heptio/ark/pkg/controller"
	arkdiscovery "github.com/heptio/ark/pkg/discovery"
	"github.com/heptio/ark/pkg/generated/clientset"
	arkv1client "github.com/heptio/ark/pkg/generated/clientset/typed/ark/v1"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions"
	"github.com/heptio/ark/pkg/restore"
	"github.com/heptio/ark/pkg/restore/restorers"
	"github.com/heptio/ark/pkg/util/kube"
	"github.com/heptio/ark/pkg/util/logging"
)

func NewCommand() *cobra.Command {
	var (
		kubeconfig      string
		sortedLogLevels = getSortedLogLevels()
		logLevelFlag    = flag.NewEnum(logrus.InfoLevel.String(), sortedLogLevels...)
	)

	var command = &cobra.Command{
		Use:   "server",
		Short: "Run the ark server",
		Long:  "Run the ark server",
		Run: func(c *cobra.Command, args []string) {
			logLevel := logrus.InfoLevel

			if parsed, err := logrus.ParseLevel(logLevelFlag.String()); err == nil {
				logLevel = parsed
			} else {
				// This should theoretically never happen assuming the enum flag
				// is constructed correctly because the enum flag will not allow
				//  an invalid value to be set.
				logrus.Errorf("log-level flag has invalid value %s", strings.ToUpper(logLevelFlag.String()))
			}
			logrus.Infof("setting log-level to %s", strings.ToUpper(logLevel.String()))

			logger := newLogger(logLevel, &logging.ErrorLocationHook{}, &logging.LogLocationHook{})

			s, err := newServer(kubeconfig, fmt.Sprintf("%s-%s", c.Parent().Name(), c.Name()), logger)

			cmd.CheckError(err)

			cmd.CheckError(s.run())
		},
	}

	command.Flags().Var(logLevelFlag, "log-level", fmt.Sprintf("the level at which to log. Valid values are %s.", strings.Join(sortedLogLevels, ", ")))
	command.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Path to the kubeconfig file to use to talk to the Kubernetes apiserver. If unset, try the environment variable KUBECONFIG, as well as in-cluster configuration")

	return command
}

func newLogger(level logrus.Level, hooks ...logrus.Hook) *logrus.Logger {
	logger := logrus.New()
	logger.Level = level

	for _, hook := range hooks {
		logger.Hooks.Add(hook)
	}

	return logger
}

// getSortedLogLevels returns a string slice containing all of the valid logrus
// log levels (based on logrus.AllLevels), sorted in ascending order of severity.
func getSortedLogLevels() []string {
	var (
		sortedLogLevels  = make([]logrus.Level, len(logrus.AllLevels))
		logLevelsStrings []string
	)

	copy(sortedLogLevels, logrus.AllLevels)

	// logrus.Panic has the lowest value, so the compare function uses ">"
	sort.Slice(sortedLogLevels, func(i, j int) bool { return sortedLogLevels[i] > sortedLogLevels[j] })

	for _, level := range sortedLogLevels {
		logLevelsStrings = append(logLevelsStrings, level.String())
	}

	return logLevelsStrings
}

type server struct {
	kubeClientConfig      *rest.Config
	kubeClient            kubernetes.Interface
	arkClient             clientset.Interface
	backupService         cloudprovider.BackupService
	snapshotService       cloudprovider.SnapshotService
	discoveryClient       discovery.DiscoveryInterface
	clientPool            dynamic.ClientPool
	sharedInformerFactory informers.SharedInformerFactory
	ctx                   context.Context
	cancelFunc            context.CancelFunc
	logger                *logrus.Logger
}

func newServer(kubeconfig, baseName string, logger *logrus.Logger) (*server, error) {
	clientConfig, err := client.Config(kubeconfig, baseName)
	if err != nil {
		return nil, err
	}

	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	arkClient, err := clientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ctx, cancelFunc := context.WithCancel(context.Background())

	s := &server{
		kubeClientConfig:      clientConfig,
		kubeClient:            kubeClient,
		arkClient:             arkClient,
		discoveryClient:       arkClient.Discovery(),
		clientPool:            dynamic.NewDynamicClientPool(clientConfig),
		sharedInformerFactory: informers.NewSharedInformerFactory(arkClient, 0),
		ctx:        ctx,
		cancelFunc: cancelFunc,
		logger:     logger,
	}

	return s, nil
}

func (s *server) run() error {
	if err := s.ensureArkNamespace(); err != nil {
		return err
	}

	originalConfig, err := s.loadConfig()
	if err != nil {
		return err
	}

	// watchConfig needs to examine the unmodified original config, so we keep that around as a
	// separate object, and instead apply defaults to a clone.
	copy, err := scheme.Scheme.DeepCopy(originalConfig)
	if err != nil {
		return errors.WithStack(err)
	}
	config := copy.(*api.Config)
	applyConfigDefaults(config, s.logger)

	s.watchConfig(originalConfig)

	if err := s.initBackupService(config); err != nil {
		return err
	}

	if err := s.initSnapshotService(config); err != nil {
		return err
	}

	if err := s.runControllers(config); err != nil {
		return err
	}

	return nil
}

func (s *server) ensureArkNamespace() error {
	logContext := s.logger.WithField("namespace", api.DefaultNamespace)

	logContext.Info("Ensuring namespace exists for backups")
	defaultNamespace := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: api.DefaultNamespace,
		},
	}

	if created, err := kube.EnsureNamespaceExists(&defaultNamespace, s.kubeClient.CoreV1().Namespaces()); created {
		logContext.Info("Namespace created")
	} else if err != nil {
		return err
	}
	logContext.Info("Namespace already exists")
	return nil
}

func (s *server) loadConfig() (*api.Config, error) {
	s.logger.Info("Retrieving Ark configuration")
	var (
		config *api.Config
		err    error
	)
	for {
		config, err = s.arkClient.ArkV1().Configs(api.DefaultNamespace).Get("default", metav1.GetOptions{})
		if err == nil {
			break
		}
		if !apierrors.IsNotFound(err) {
			s.logger.WithError(err).Error("Error retrieving configuration")
		} else {
			s.logger.Info("Configuration not found")
		}
		s.logger.Info("Will attempt to retrieve configuration again in 5 seconds")
		time.Sleep(5 * time.Second)
	}
	s.logger.Info("Successfully retrieved Ark configuration")
	return config, nil
}

const (
	defaultGCSyncPeriod       = 60 * time.Minute
	defaultBackupSyncPeriod   = 60 * time.Minute
	defaultScheduleSyncPeriod = time.Minute
)

var defaultResourcePriorities = []string{
	"namespaces",
	"persistentvolumes",
	"persistentvolumeclaims",
	"secrets",
	"configmaps",
}

func applyConfigDefaults(c *api.Config, logger *logrus.Logger) {
	if c.GCSyncPeriod.Duration == 0 {
		c.GCSyncPeriod.Duration = defaultGCSyncPeriod
	}

	if c.BackupSyncPeriod.Duration == 0 {
		c.BackupSyncPeriod.Duration = defaultBackupSyncPeriod
	}

	if c.ScheduleSyncPeriod.Duration == 0 {
		c.ScheduleSyncPeriod.Duration = defaultScheduleSyncPeriod
	}

	if len(c.ResourcePriorities) == 0 {
		c.ResourcePriorities = defaultResourcePriorities
		logger.WithField("priorities", c.ResourcePriorities).Info("Using default resource priorities")
	} else {
		logger.WithField("priorities", c.ResourcePriorities).Info("Using resource priorities from config")
	}
}

// watchConfig adds an update event handler to the Config shared informer, invoking s.cancelFunc
// when it sees a change.
func (s *server) watchConfig(config *api.Config) {
	s.sharedInformerFactory.Ark().V1().Configs().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			updated := newObj.(*api.Config)
			s.logger.WithField("name", kube.NamespaceAndName(updated)).Debug("received updated config")

			if updated.Name != config.Name {
				s.logger.WithField("name", updated.Name).Debug("Config watch channel received other config")
				return
			}

			// Objects retrieved via Get() don't have their Kind or APIVersion set. Objects retrieved via
			// Watch(), including those from shared informer event handlers, DO have their Kind and
			// APIVersion set. To prevent the DeepEqual() call below from considering Kind or APIVersion
			// as the source of a change, set config.Kind and config.APIVersion to match the values from
			// the updated Config.
			if config.Kind != updated.Kind {
				config.Kind = updated.Kind
			}
			if config.APIVersion != updated.APIVersion {
				config.APIVersion = updated.APIVersion
			}

			if !reflect.DeepEqual(config, updated) {
				s.logger.Info("Detected a config change. Gracefully shutting down")
				s.cancelFunc()
			}
		},
	})
}

func (s *server) initBackupService(config *api.Config) error {
	s.logger.Info("Configuring cloud provider for backup service")
	objectStorage, err := getObjectStorageProvider(config.BackupStorageProvider.CloudProviderConfig, "backupStorageProvider", s.logger)
	if err != nil {
		return err
	}

	s.backupService = cloudprovider.NewBackupService(objectStorage, s.logger)
	return nil
}

func (s *server) initSnapshotService(config *api.Config) error {
	if config.PersistentVolumeProvider == nil {
		s.logger.Info("PersistentVolumeProvider config not provided, volume snapshots and restores are disabled")
		return nil
	}

	s.logger.Info("Configuring cloud provider for snapshot service")
	blockStorage, err := getBlockStorageProvider(*config.PersistentVolumeProvider, "persistentVolumeProvider")
	if err != nil {
		return err
	}
	s.snapshotService = cloudprovider.NewSnapshotService(blockStorage)
	return nil
}

func hasOneCloudProvider(cloudConfig api.CloudProviderConfig) bool {
	found := false

	if cloudConfig.AWS != nil {
		found = true
	}

	if cloudConfig.GCP != nil {
		if found {
			return false
		}
		found = true
	}

	if cloudConfig.Azure != nil {
		if found {
			return false
		}
		found = true
	}

	return found
}

func getObjectStorageProvider(cloudConfig api.CloudProviderConfig, field string, logger *logrus.Logger) (cloudprovider.ObjectStorageAdapter, error) {
	var (
		objectStorage cloudprovider.ObjectStorageAdapter
		err           error
	)

	if !hasOneCloudProvider(cloudConfig) {
		return nil, errors.Errorf("you must specify exactly one of aws, gcp, or azure for %s", field)
	}

	switch {
	case cloudConfig.AWS != nil:
		objectStorage, err = arkaws.NewObjectStorageAdapter(
			cloudConfig.AWS.Region,
			cloudConfig.AWS.S3Url,
			cloudConfig.AWS.KMSKeyID,
			cloudConfig.AWS.S3ForcePathStyle)
	case cloudConfig.GCP != nil:
		var email string
		var privateKey []byte

		credentialsFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
		if credentialsFile != "" {
			// Get the email and private key from the credentials file so we can pre-sign download URLs
			creds, err := ioutil.ReadFile(credentialsFile)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			jwtConfig, err := google.JWTConfigFromJSON(creds)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			email = jwtConfig.Email
			privateKey = jwtConfig.PrivateKey
		} else {
			logger.Warning("GOOGLE_APPLICATION_CREDENTIALS is undefined; some features such as downloading log files will not work")
		}

		objectStorage, err = gcp.NewObjectStorageAdapter(email, privateKey)
	case cloudConfig.Azure != nil:
		objectStorage, err = azure.NewObjectStorageAdapter()
	}

	if err != nil {
		return nil, err
	}

	return objectStorage, nil
}

func getBlockStorageProvider(cloudConfig api.CloudProviderConfig, field string) (cloudprovider.BlockStorageAdapter, error) {
	var (
		blockStorage cloudprovider.BlockStorageAdapter
		err          error
	)

	if !hasOneCloudProvider(cloudConfig) {
		return nil, errors.Errorf("you must specify exactly one of aws, gcp, or azure for %s", field)
	}

	switch {
	case cloudConfig.AWS != nil:
		blockStorage, err = arkaws.NewBlockStorageAdapter(cloudConfig.AWS.Region)
	case cloudConfig.GCP != nil:
		blockStorage, err = gcp.NewBlockStorageAdapter(cloudConfig.GCP.Project)
	case cloudConfig.Azure != nil:
		blockStorage, err = azure.NewBlockStorageAdapter(cloudConfig.Azure.Location, cloudConfig.Azure.APITimeout.Duration)
	}

	if err != nil {
		return nil, err
	}

	return blockStorage, nil
}

func durationMin(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func (s *server) runControllers(config *api.Config) error {
	s.logger.Info("Starting controllers")

	ctx := s.ctx
	var wg sync.WaitGroup

	cloudBackupCacheResyncPeriod := durationMin(config.GCSyncPeriod.Duration, config.BackupSyncPeriod.Duration)
	s.logger.Infof("Caching cloud backups every %s", cloudBackupCacheResyncPeriod)
	s.backupService = cloudprovider.NewBackupServiceWithCachedBackupGetter(
		ctx,
		s.backupService,
		cloudBackupCacheResyncPeriod,
		s.logger,
	)

	backupSyncController := controller.NewBackupSyncController(
		s.arkClient.ArkV1(),
		s.backupService,
		config.BackupStorageProvider.Bucket,
		config.BackupSyncPeriod.Duration,
		s.logger,
	)
	wg.Add(1)
	go func() {
		backupSyncController.Run(ctx, 1)
		wg.Done()
	}()

	discoveryHelper, err := arkdiscovery.NewHelper(s.discoveryClient, s.logger)
	if err != nil {
		return err
	}
	go wait.Until(
		func() {
			if err := discoveryHelper.Refresh(); err != nil {
				s.logger.WithError(err).Error("Error refreshing discovery")
			}
		},
		5*time.Minute,
		ctx.Done(),
	)

	if config.RestoreOnlyMode {
		s.logger.Info("Restore only mode - not starting the backup, schedule or GC controllers")
	} else {
		backupper, err := newBackupper(discoveryHelper, s.clientPool, s.backupService, s.snapshotService, s.kubeClientConfig, s.kubeClient.CoreV1())
		cmd.CheckError(err)
		backupController := controller.NewBackupController(
			s.sharedInformerFactory.Ark().V1().Backups(),
			s.arkClient.ArkV1(),
			backupper,
			s.backupService,
			config.BackupStorageProvider.Bucket,
			s.snapshotService != nil,
			s.logger,
		)
		wg.Add(1)
		go func() {
			backupController.Run(ctx, 1)
			wg.Done()
		}()

		scheduleController := controller.NewScheduleController(
			s.arkClient.ArkV1(),
			s.arkClient.ArkV1(),
			s.sharedInformerFactory.Ark().V1().Schedules(),
			config.ScheduleSyncPeriod.Duration,
			s.logger,
		)
		wg.Add(1)
		go func() {
			scheduleController.Run(ctx, 1)
			wg.Done()
		}()

		gcController := controller.NewGCController(
			s.backupService,
			s.snapshotService,
			config.BackupStorageProvider.Bucket,
			config.GCSyncPeriod.Duration,
			s.sharedInformerFactory.Ark().V1().Backups(),
			s.arkClient.ArkV1(),
			s.sharedInformerFactory.Ark().V1().Restores(),
			s.arkClient.ArkV1(),
			s.logger,
		)
		wg.Add(1)
		go func() {
			gcController.Run(ctx, 1)
			wg.Done()
		}()
	}

	restorer, err := newRestorer(
		discoveryHelper,
		s.clientPool,
		s.backupService,
		s.snapshotService,
		config.ResourcePriorities,
		s.arkClient.ArkV1(),
		s.kubeClient,
		s.logger,
	)
	cmd.CheckError(err)

	restoreController := controller.NewRestoreController(
		s.sharedInformerFactory.Ark().V1().Restores(),
		s.arkClient.ArkV1(),
		s.arkClient.ArkV1(),
		restorer,
		s.backupService,
		config.BackupStorageProvider.Bucket,
		s.sharedInformerFactory.Ark().V1().Backups(),
		s.snapshotService != nil,
		s.logger,
	)
	wg.Add(1)
	go func() {
		restoreController.Run(ctx, 1)
		wg.Done()
	}()

	downloadRequestController := controller.NewDownloadRequestController(
		s.arkClient.ArkV1(),
		s.sharedInformerFactory.Ark().V1().DownloadRequests(),
		s.backupService,
		config.BackupStorageProvider.Bucket,
		s.logger,
	)
	wg.Add(1)
	go func() {
		downloadRequestController.Run(ctx, 1)
		wg.Done()
	}()

	// SHARED INFORMERS HAVE TO BE STARTED AFTER ALL CONTROLLERS
	go s.sharedInformerFactory.Start(ctx.Done())

	s.logger.Info("Server started successfully")

	<-ctx.Done()

	s.logger.Info("Waiting for all controllers to shut down gracefully")
	wg.Wait()

	return nil
}

func newBackupper(
	discoveryHelper arkdiscovery.Helper,
	clientPool dynamic.ClientPool,
	backupService cloudprovider.BackupService,
	snapshotService cloudprovider.SnapshotService,
	kubeClientConfig *rest.Config,
	kubeCoreV1Client kcorev1client.CoreV1Interface,
) (backup.Backupper, error) {
	actions := map[string]backup.Action{}
	dynamicFactory := client.NewDynamicFactory(clientPool)

	if snapshotService != nil {
		action, err := backup.NewVolumeSnapshotAction(snapshotService)
		if err != nil {
			return nil, err
		}
		actions["persistentvolumes"] = action

		actions["persistentvolumeclaims"] = backup.NewBackupPVAction()
	}

	return backup.NewKubernetesBackupper(
		discoveryHelper,
		dynamicFactory,
		actions,
		backup.NewPodCommandExecutor(kubeClientConfig, kubeCoreV1Client.RESTClient()),
	)
}

func newRestorer(
	discoveryHelper arkdiscovery.Helper,
	clientPool dynamic.ClientPool,
	backupService cloudprovider.BackupService,
	snapshotService cloudprovider.SnapshotService,
	resourcePriorities []string,
	backupClient arkv1client.BackupsGetter,
	kubeClient kubernetes.Interface,
	logger *logrus.Logger,
) (restore.Restorer, error) {
	restorers := map[string]restorers.ResourceRestorer{
		"persistentvolumes":      restorers.NewPersistentVolumeRestorer(snapshotService),
		"persistentvolumeclaims": restorers.NewPersistentVolumeClaimRestorer(),
		"services":               restorers.NewServiceRestorer(),
		"namespaces":             restorers.NewNamespaceRestorer(),
		"pods":                   restorers.NewPodRestorer(logger),
		"jobs":                   restorers.NewJobRestorer(logger),
	}

	return restore.NewKubernetesRestorer(
		discoveryHelper,
		client.NewDynamicFactory(clientPool),
		restorers,
		backupService,
		resourcePriorities,
		backupClient,
		kubeClient.CoreV1().Namespaces(),
		logger,
	)
}
