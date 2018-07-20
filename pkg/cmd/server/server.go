/*
Copyright 2017 the Heptio Ark contributors.

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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	kubeerrs "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/backup"
	"github.com/heptio/ark/pkg/buildinfo"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/cmd"
	"github.com/heptio/ark/pkg/cmd/util/signals"
	"github.com/heptio/ark/pkg/controller"
	arkdiscovery "github.com/heptio/ark/pkg/discovery"
	clientset "github.com/heptio/ark/pkg/generated/clientset/versioned"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions"
	"github.com/heptio/ark/pkg/metrics"
	"github.com/heptio/ark/pkg/plugin"
	"github.com/heptio/ark/pkg/podexec"
	"github.com/heptio/ark/pkg/restic"
	"github.com/heptio/ark/pkg/restore"
	"github.com/heptio/ark/pkg/util/kube"
	"github.com/heptio/ark/pkg/util/logging"
	"github.com/heptio/ark/pkg/util/stringslice"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	// the port where prometheus metrics are exposed
	defaultMetricsAddress = ":8085"
)

func NewCommand() *cobra.Command {
	var (
		logLevelFlag   = logging.LogLevelFlag(logrus.InfoLevel)
		pluginDir      = "/plugins"
		metricsAddress = defaultMetricsAddress
	)

	var command = &cobra.Command{
		Use:   "server",
		Short: "Run the ark server",
		Long:  "Run the ark server",
		Run: func(c *cobra.Command, args []string) {
			// go-plugin uses log.Println to log when it's waiting for all plugin processes to complete so we need to
			// set its output to stdout.
			log.SetOutput(os.Stdout)

			logLevel := logLevelFlag.Parse()
			// Make sure we log to stdout so cloud log dashboards don't show this as an error.
			logrus.SetOutput(os.Stdout)
			logrus.Infof("setting log-level to %s", strings.ToUpper(logLevel.String()))

			// Ark's DefaultLogger logs to stdout, so all is good there.
			logger := logging.DefaultLogger(logLevel)
			logger.Infof("Starting Ark server %s", buildinfo.FormattedGitSHA())

			// NOTE: the namespace flag is bound to ark's persistent flags when the root ark command
			// creates the client Factory and binds the Factory's flags. We're not using a Factory here in
			// the server because the Factory gets its basename set at creation time, and the basename is
			// used to construct the user-agent for clients. Also, the Factory's Namespace() method uses
			// the client config file to determine the appropriate namespace to use, and that isn't
			// applicable to the server (it uses the method directly below instead). We could potentially
			// add a SetBasename() method to the Factory, and tweak how Namespace() works, if we wanted to
			// have the server use the Factory.
			namespaceFlag := c.Flag("namespace")
			if namespaceFlag == nil {
				cmd.CheckError(errors.New("unable to look up namespace flag"))
			}
			namespace := getServerNamespace(namespaceFlag)

			s, err := newServer(namespace, fmt.Sprintf("%s-%s", c.Parent().Name(), c.Name()), pluginDir, metricsAddress, logger)
			cmd.CheckError(err)

			cmd.CheckError(s.run())
		},
	}

	command.Flags().Var(logLevelFlag, "log-level", fmt.Sprintf("the level at which to log. Valid values are %s.", strings.Join(logLevelFlag.AllowedValues(), ", ")))
	command.Flags().StringVar(&pluginDir, "plugin-dir", pluginDir, "directory containing Ark plugins")
	command.Flags().StringVar(&metricsAddress, "metrics-address", metricsAddress, "the address to expose prometheus metrics")

	return command
}

func getServerNamespace(namespaceFlag *pflag.Flag) string {
	if namespaceFlag.Changed {
		return namespaceFlag.Value.String()
	}

	if data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns
		}
	}

	return api.DefaultNamespace
}

type server struct {
	namespace             string
	metricsAddress        string
	kubeClientConfig      *rest.Config
	kubeClient            kubernetes.Interface
	arkClient             clientset.Interface
	objectStore           cloudprovider.ObjectStore
	backupService         cloudprovider.BackupService
	snapshotService       cloudprovider.SnapshotService
	discoveryClient       discovery.DiscoveryInterface
	clientPool            dynamic.ClientPool
	discoveryHelper       arkdiscovery.Helper
	sharedInformerFactory informers.SharedInformerFactory
	ctx                   context.Context
	cancelFunc            context.CancelFunc
	logger                logrus.FieldLogger
	pluginManager         plugin.Manager
	resticManager         restic.RepositoryManager
	metrics               *metrics.ServerMetrics
}

func newServer(namespace, baseName, pluginDir, metricsAddr string, logger *logrus.Logger) (*server, error) {
	clientConfig, err := client.Config("", "", baseName)
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

	pluginManager, err := plugin.NewManager(logger, logger.Level, pluginDir)
	if err != nil {
		return nil, err
	}

	ctx, cancelFunc := context.WithCancel(context.Background())

	s := &server{
		namespace:             namespace,
		metricsAddress:        metricsAddr,
		kubeClientConfig:      clientConfig,
		kubeClient:            kubeClient,
		arkClient:             arkClient,
		discoveryClient:       arkClient.Discovery(),
		clientPool:            dynamic.NewDynamicClientPool(clientConfig),
		sharedInformerFactory: informers.NewFilteredSharedInformerFactory(arkClient, 0, namespace, nil),
		ctx:           ctx,
		cancelFunc:    cancelFunc,
		logger:        logger,
		pluginManager: pluginManager,
	}

	return s, nil
}

func (s *server) run() error {
	defer s.pluginManager.CleanupClients()

	signals.CancelOnShutdown(s.cancelFunc, s.logger)

	// Since s.namespace, which specifies where backups/restores/schedules/etc. should live,
	// *could* be different from the namespace where the Ark server pod runs, check to make
	// sure it exists, and fail fast if it doesn't.
	if err := s.namespaceExists(s.namespace); err != nil {
		return err
	}

	if err := s.initDiscoveryHelper(); err != nil {
		return err
	}

	// check to ensure all Ark CRDs exist
	if err := s.arkResourcesExist(); err != nil {
		return err
	}

	originalConfig, err := s.loadConfig()
	if err != nil {
		return err
	}

	// watchConfig needs to examine the unmodified original config, so we keep that around as a
	// separate object, and instead apply defaults to a clone.
	config := originalConfig.DeepCopy()
	applyConfigDefaults(config, s.logger)

	s.watchConfig(originalConfig)

	if err := s.initBackupService(config); err != nil {
		return err
	}

	if err := s.initSnapshotService(config); err != nil {
		return err
	}

	if config.BackupStorageProvider.ResticLocation != "" {
		if err := s.initRestic(config.BackupStorageProvider); err != nil {
			return err
		}
	}

	if err := s.runControllers(config); err != nil {
		return err
	}

	return nil
}

// namespaceExists returns nil if namespace can be successfully
// gotten from the kubernetes API, or an error otherwise.
func (s *server) namespaceExists(namespace string) error {
	s.logger.WithField("namespace", namespace).Info("Checking existence of namespace")

	if _, err := s.kubeClient.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{}); err != nil {
		return errors.WithStack(err)
	}

	s.logger.WithField("namespace", namespace).Info("Namespace exists")
	return nil
}

// initDiscoveryHelper instantiates the server's discovery helper and spawns a
// goroutine to call Refresh() every 5 minutes.
func (s *server) initDiscoveryHelper() error {
	discoveryHelper, err := arkdiscovery.NewHelper(s.discoveryClient, s.logger)
	if err != nil {
		return err
	}
	s.discoveryHelper = discoveryHelper

	go wait.Until(
		func() {
			if err := discoveryHelper.Refresh(); err != nil {
				s.logger.WithError(err).Error("Error refreshing discovery")
			}
		},
		5*time.Minute,
		s.ctx.Done(),
	)

	return nil
}

// arkResourcesExist checks for the existence of each Ark CRD via discovery
// and returns an error if any of them don't exist.
func (s *server) arkResourcesExist() error {
	s.logger.Info("Checking existence of Ark custom resource definitions")

	var arkGroupVersion *metav1.APIResourceList
	for _, gv := range s.discoveryHelper.Resources() {
		if gv.GroupVersion == api.SchemeGroupVersion.String() {
			arkGroupVersion = gv
			break
		}
	}

	if arkGroupVersion == nil {
		return errors.Errorf("Ark API group %s not found", api.SchemeGroupVersion)
	}

	foundResources := sets.NewString()
	for _, resource := range arkGroupVersion.APIResources {
		foundResources.Insert(resource.Kind)
	}

	var errs []error
	for kind := range api.CustomResources() {
		if foundResources.Has(kind) {
			s.logger.WithField("kind", kind).Debug("Found custom resource")
			continue
		}

		errs = append(errs, errors.Errorf("custom resource %s not found in Ark API group %s", kind, api.SchemeGroupVersion))
	}

	if len(errs) > 0 {
		return kubeerrs.NewAggregate(errs)
	}

	s.logger.Info("All Ark custom resource definitions exist")
	return nil
}

func (s *server) loadConfig() (*api.Config, error) {
	s.logger.Info("Retrieving Ark configuration")
	var (
		config *api.Config
		err    error
	)
	for {
		config, err = s.arkClient.ArkV1().Configs(s.namespace).Get("default", metav1.GetOptions{})
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
	defaultGCSyncPeriod              = 60 * time.Minute
	defaultBackupSyncPeriod          = 60 * time.Minute
	defaultScheduleSyncPeriod        = time.Minute
	defaultPodVolumeOperationTimeout = 60 * time.Minute
)

// - Namespaces go first because all namespaced resources depend on them.
// - PVs go before PVCs because PVCs depend on them.
// - PVCs go before pods or controllers so they can be mounted as volumes.
// - Secrets and config maps go before pods or controllers so they can be mounted
// 	 as volumes.
// - Service accounts go before pods or controllers so pods can use them.
// - Limit ranges go before pods or controllers so pods can use them.
// - Pods go before controllers so they can be explicitly restored and potentially
//	 have restic restores run before controllers adopt the pods.
var defaultResourcePriorities = []string{
	"namespaces",
	"persistentvolumes",
	"persistentvolumeclaims",
	"secrets",
	"configmaps",
	"serviceaccounts",
	"limitranges",
	"pods",
}

func applyConfigDefaults(c *api.Config, logger logrus.FieldLogger) {
	if c.GCSyncPeriod.Duration == 0 {
		c.GCSyncPeriod.Duration = defaultGCSyncPeriod
	}

	if c.BackupSyncPeriod.Duration == 0 {
		c.BackupSyncPeriod.Duration = defaultBackupSyncPeriod
	}

	if c.ScheduleSyncPeriod.Duration == 0 {
		c.ScheduleSyncPeriod.Duration = defaultScheduleSyncPeriod
	}

	if c.PodVolumeOperationTimeout.Duration == 0 {
		c.PodVolumeOperationTimeout.Duration = defaultPodVolumeOperationTimeout
	}

	if len(c.ResourcePriorities) == 0 {
		c.ResourcePriorities = defaultResourcePriorities
		logger.WithField("priorities", c.ResourcePriorities).Info("Using default resource priorities")
	} else {
		logger.WithField("priorities", c.ResourcePriorities).Info("Using resource priorities from config")
	}

	if c.BackupStorageProvider.Config == nil {
		c.BackupStorageProvider.Config = make(map[string]string)
	}

	// add the bucket name to the config map so that object stores can use
	// it when initializing. The AWS object store uses this to determine the
	// bucket's region when setting up its client.
	c.BackupStorageProvider.Config["bucket"] = c.BackupStorageProvider.Bucket
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
	objectStore, err := getObjectStore(config.BackupStorageProvider.CloudProviderConfig, s.pluginManager)
	if err != nil {
		return err
	}

	s.objectStore = objectStore
	s.backupService = cloudprovider.NewBackupService(objectStore, s.logger)
	return nil
}

func (s *server) initSnapshotService(config *api.Config) error {
	if config.PersistentVolumeProvider == nil {
		s.logger.Info("PersistentVolumeProvider config not provided, volume snapshots and restores are disabled")
		return nil
	}

	s.logger.Info("Configuring cloud provider for snapshot service")
	blockStore, err := getBlockStore(*config.PersistentVolumeProvider, s.pluginManager)
	if err != nil {
		return err
	}
	s.snapshotService = cloudprovider.NewSnapshotService(blockStore)
	return nil
}

func getObjectStore(cloudConfig api.CloudProviderConfig, manager plugin.Manager) (cloudprovider.ObjectStore, error) {
	if cloudConfig.Name == "" {
		return nil, errors.New("object storage provider name must not be empty")
	}

	objectStore, err := manager.GetObjectStore(cloudConfig.Name)
	if err != nil {
		return nil, err
	}

	if err := objectStore.Init(cloudConfig.Config); err != nil {
		return nil, err
	}

	return objectStore, nil
}

func getBlockStore(cloudConfig api.CloudProviderConfig, manager plugin.Manager) (cloudprovider.BlockStore, error) {
	if cloudConfig.Name == "" {
		return nil, errors.New("block storage provider name must not be empty")
	}

	blockStore, err := manager.GetBlockStore(cloudConfig.Name)
	if err != nil {
		return nil, err
	}

	if err := blockStore.Init(cloudConfig.Config); err != nil {
		return nil, err
	}

	return blockStore, nil
}

func durationMin(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func (s *server) initRestic(config api.ObjectStorageProviderConfig) error {
	// warn if restic daemonset does not exist
	if _, err := s.kubeClient.AppsV1().DaemonSets(s.namespace).Get(restic.DaemonSet, metav1.GetOptions{}); apierrors.IsNotFound(err) {
		s.logger.Warn("Ark restic daemonset not found; restic backups/restores will not work until it's created")
	} else if err != nil {
		s.logger.WithError(errors.WithStack(err)).Warn("Error checking for existence of ark restic daemonset")
	}

	// ensure the repo key secret is set up
	if err := restic.EnsureCommonRepositoryKey(s.kubeClient.CoreV1(), s.namespace); err != nil {
		return err
	}

	// set the env vars that restic uses for creds purposes
	if config.Name == string(restic.AzureBackend) {
		os.Setenv("AZURE_ACCOUNT_NAME", os.Getenv("AZURE_STORAGE_ACCOUNT_ID"))
		os.Setenv("AZURE_ACCOUNT_KEY", os.Getenv("AZURE_STORAGE_KEY"))
	}

	// use a stand-alone secrets informer so we can filter to only the restic credentials
	// secret(s) within the heptio-ark namespace
	//
	// note: using an informer to access the single secret for all ark-managed
	// restic repositories is overkill for now, but will be useful when we move
	// to fully-encrypted backups and have unique keys per repository.
	secretsInformer := corev1informers.NewFilteredSecretInformer(
		s.kubeClient,
		s.namespace,
		0,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		func(opts *metav1.ListOptions) {
			opts.FieldSelector = fmt.Sprintf("metadata.name=%s", restic.CredentialsSecretName)
		},
	)
	go secretsInformer.Run(s.ctx.Done())

	res, err := restic.NewRepositoryManager(
		s.ctx,
		s.namespace,
		s.arkClient,
		secretsInformer,
		s.sharedInformerFactory.Ark().V1().ResticRepositories(),
		s.arkClient.ArkV1(),
		s.logger,
	)
	if err != nil {
		return err
	}
	s.resticManager = res

	return nil
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

	go func() {
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", promhttp.Handler())
		s.logger.Infof("Starting metric server at address [%s]", s.metricsAddress)
		if err := http.ListenAndServe(s.metricsAddress, metricsMux); err != nil {
			s.logger.Fatalf("Failed to start metric server at [%s]: %v", s.metricsAddress, err)
		}
	}()
	s.metrics = metrics.NewServerMetrics()
	s.metrics.RegisterAllMetrics()

	backupSyncController := controller.NewBackupSyncController(
		s.arkClient.ArkV1(),
		s.backupService,
		config.BackupStorageProvider.Bucket,
		config.BackupSyncPeriod.Duration,
		s.namespace,
		s.logger,
	)
	wg.Add(1)
	go func() {
		backupSyncController.Run(ctx, 1)
		wg.Done()
	}()

	if config.RestoreOnlyMode {
		s.logger.Info("Restore only mode - not starting the backup, schedule, delete-backup, or GC controllers")
	} else {
		backupTracker := controller.NewBackupTracker()

		backupper, err := backup.NewKubernetesBackupper(
			s.discoveryHelper,
			client.NewDynamicFactory(s.clientPool),
			podexec.NewPodCommandExecutor(s.kubeClientConfig, s.kubeClient.CoreV1().RESTClient()),
			s.snapshotService,
			s.resticManager,
			config.PodVolumeOperationTimeout.Duration,
		)
		cmd.CheckError(err)

		backupController := controller.NewBackupController(
			s.sharedInformerFactory.Ark().V1().Backups(),
			s.arkClient.ArkV1(),
			backupper,
			s.backupService,
			config.BackupStorageProvider.Bucket,
			s.snapshotService != nil,
			s.logger,
			s.pluginManager,
			backupTracker,
			s.metrics,
		)
		wg.Add(1)
		go func() {
			backupController.Run(ctx, 1)
			wg.Done()
		}()

		scheduleController := controller.NewScheduleController(
			s.namespace,
			s.arkClient.ArkV1(),
			s.arkClient.ArkV1(),
			s.sharedInformerFactory.Ark().V1().Schedules(),
			config.ScheduleSyncPeriod.Duration,
			s.logger,
			s.metrics,
		)
		wg.Add(1)
		go func() {
			scheduleController.Run(ctx, 1)
			wg.Done()
		}()

		gcController := controller.NewGCController(
			s.logger,
			s.sharedInformerFactory.Ark().V1().Backups(),
			s.sharedInformerFactory.Ark().V1().DeleteBackupRequests(),
			s.arkClient.ArkV1(),
			config.GCSyncPeriod.Duration,
		)
		wg.Add(1)
		go func() {
			gcController.Run(ctx, 1)
			wg.Done()
		}()

		backupDeletionController := controller.NewBackupDeletionController(
			s.logger,
			s.sharedInformerFactory.Ark().V1().DeleteBackupRequests(),
			s.arkClient.ArkV1(), // deleteBackupRequestClient
			s.arkClient.ArkV1(), // backupClient
			s.snapshotService,
			s.backupService,
			config.BackupStorageProvider.Bucket,
			s.sharedInformerFactory.Ark().V1().Restores(),
			s.arkClient.ArkV1(), // restoreClient
			backupTracker,
			s.resticManager,
			s.sharedInformerFactory.Ark().V1().PodVolumeBackups(),
		)
		wg.Add(1)
		go func() {
			backupDeletionController.Run(ctx, 1)
			wg.Done()
		}()

	}

	restorer, err := restore.NewKubernetesRestorer(
		s.discoveryHelper,
		client.NewDynamicFactory(s.clientPool),
		s.backupService,
		s.snapshotService,
		config.ResourcePriorities,
		s.arkClient.ArkV1(),
		s.kubeClient.CoreV1().Namespaces(),
		s.resticManager,
		config.PodVolumeOperationTimeout.Duration,
		s.logger,
	)
	cmd.CheckError(err)

	restoreController := controller.NewRestoreController(
		s.namespace,
		s.sharedInformerFactory.Ark().V1().Restores(),
		s.arkClient.ArkV1(),
		s.arkClient.ArkV1(),
		restorer,
		s.backupService,
		config.BackupStorageProvider.Bucket,
		s.sharedInformerFactory.Ark().V1().Backups(),
		s.snapshotService != nil,
		s.logger,
		s.pluginManager,
	)
	wg.Add(1)
	go func() {
		restoreController.Run(ctx, 1)
		wg.Done()
	}()

	downloadRequestController := controller.NewDownloadRequestController(
		s.arkClient.ArkV1(),
		s.sharedInformerFactory.Ark().V1().DownloadRequests(),
		s.sharedInformerFactory.Ark().V1().Restores(),
		s.backupService,
		config.BackupStorageProvider.Bucket,
		s.logger,
	)
	wg.Add(1)
	go func() {
		downloadRequestController.Run(ctx, 1)
		wg.Done()
	}()

	if s.resticManager != nil {
		resticRepoController := controller.NewResticRepositoryController(
			s.logger,
			s.sharedInformerFactory.Ark().V1().ResticRepositories(),
			s.arkClient.ArkV1(),
			config.BackupStorageProvider,
			s.resticManager,
		)
		wg.Add(1)
		go func() {
			// TODO only having a single worker may be an issue since maintenance
			// can take a long time.
			resticRepoController.Run(ctx, 1)
			wg.Done()
		}()
	}

	// SHARED INFORMERS HAVE TO BE STARTED AFTER ALL CONTROLLERS
	go s.sharedInformerFactory.Start(ctx.Done())

	// Remove this sometime after v0.8.0
	cache.WaitForCacheSync(ctx.Done(), s.sharedInformerFactory.Ark().V1().Backups().Informer().HasSynced)
	s.removeDeprecatedGCFinalizer()

	s.logger.Info("Server started successfully")

	<-ctx.Done()

	s.logger.Info("Waiting for all controllers to shut down gracefully")
	wg.Wait()

	return nil
}

const gcFinalizer = "gc.ark.heptio.com"

func (s *server) removeDeprecatedGCFinalizer() {
	backups, err := s.sharedInformerFactory.Ark().V1().Backups().Lister().List(labels.Everything())
	if err != nil {
		s.logger.WithError(errors.WithStack(err)).Error("error listing backups from cache - unable to remove old finalizers")
		return
	}

	for _, backup := range backups {
		log := s.logger.WithField("backup", kube.NamespaceAndName(backup))

		if !stringslice.Has(backup.Finalizers, gcFinalizer) {
			log.Debug("backup doesn't have deprecated finalizer - skipping")
			continue
		}

		log.Info("removing deprecated finalizer from backup")

		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"finalizers":      stringslice.Except(backup.Finalizers, gcFinalizer),
				"resourceVersion": backup.ResourceVersion,
			},
		}

		patchBytes, err := json.Marshal(patch)
		if err != nil {
			log.WithError(errors.WithStack(err)).Error("error marshaling finalizers patch")
			continue
		}

		_, err = s.arkClient.ArkV1().Backups(backup.Namespace).Patch(backup.Name, types.MergePatchType, patchBytes)
		if err != nil {
			log.WithError(errors.WithStack(err)).Error("error marshaling finalizers patch")
		}
	}
}
