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
	"net/http/pprof"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	api "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/backup"
	"github.com/heptio/velero/pkg/buildinfo"
	"github.com/heptio/velero/pkg/client"
	"github.com/heptio/velero/pkg/cmd"
	"github.com/heptio/velero/pkg/cmd/util/flag"
	"github.com/heptio/velero/pkg/cmd/util/signals"
	"github.com/heptio/velero/pkg/controller"
	velerodiscovery "github.com/heptio/velero/pkg/discovery"
	clientset "github.com/heptio/velero/pkg/generated/clientset/versioned"
	informers "github.com/heptio/velero/pkg/generated/informers/externalversions"
	"github.com/heptio/velero/pkg/metrics"
	"github.com/heptio/velero/pkg/persistence"
	"github.com/heptio/velero/pkg/plugin"
	"github.com/heptio/velero/pkg/podexec"
	"github.com/heptio/velero/pkg/restic"
	"github.com/heptio/velero/pkg/restore"
	"github.com/heptio/velero/pkg/util/kube"
	"github.com/heptio/velero/pkg/util/logging"
	"github.com/heptio/velero/pkg/util/stringslice"
)

const (
	// the port where prometheus metrics are exposed
	defaultMetricsAddress = ":8085"

	defaultBackupSyncPeriod           = time.Minute
	defaultPodVolumeOperationTimeout  = 60 * time.Minute
	defaultResourceTerminatingTimeout = 10 * time.Minute

	// server's client default qps and burst
	defaultClientQPS   float32 = 20.0
	defaultClientBurst int     = 30

	defaultProfilerAddress = "localhost:6060"
)

type serverConfig struct {
	pluginDir, metricsAddress, defaultBackupLocation                        string
	backupSyncPeriod, podVolumeOperationTimeout, resourceTerminatingTimeout time.Duration
	restoreResourcePriorities                                               []string
	defaultVolumeSnapshotLocations                                          map[string]string
	restoreOnly                                                             bool
	clientQPS                                                               float32
	clientBurst                                                             int
	profilerAddress                                                         string
}

func NewCommand() *cobra.Command {
	var (
		volumeSnapshotLocations = flag.NewMap().WithKeyValueDelimiter(":")
		logLevelFlag            = logging.LogLevelFlag(logrus.InfoLevel)
		config                  = serverConfig{
			pluginDir:                      "/plugins",
			metricsAddress:                 defaultMetricsAddress,
			defaultBackupLocation:          "default",
			defaultVolumeSnapshotLocations: make(map[string]string),
			backupSyncPeriod:               defaultBackupSyncPeriod,
			podVolumeOperationTimeout:      defaultPodVolumeOperationTimeout,
			restoreResourcePriorities:      defaultRestorePriorities,
			clientQPS:                      defaultClientQPS,
			clientBurst:                    defaultClientBurst,
			profilerAddress:                defaultProfilerAddress,
			resourceTerminatingTimeout:     defaultResourceTerminatingTimeout,
		}
	)

	var command = &cobra.Command{
		Use:   "server",
		Short: "Run the velero server",
		Long:  "Run the velero server",
		Run: func(c *cobra.Command, args []string) {
			// go-plugin uses log.Println to log when it's waiting for all plugin processes to complete so we need to
			// set its output to stdout.
			log.SetOutput(os.Stdout)

			logLevel := logLevelFlag.Parse()
			// Make sure we log to stdout so cloud log dashboards don't show this as an error.
			logrus.SetOutput(os.Stdout)
			logrus.Infof("setting log-level to %s", strings.ToUpper(logLevel.String()))

			// Velero's DefaultLogger logs to stdout, so all is good there.
			logger := logging.DefaultLogger(logLevel)
			logger.Infof("Starting Velero server %s (%s)", buildinfo.Version, buildinfo.FormattedGitSHA())

			// NOTE: the namespace flag is bound to velero's persistent flags when the root velero command
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

			if volumeSnapshotLocations.Data() != nil {
				config.defaultVolumeSnapshotLocations = volumeSnapshotLocations.Data()
			}

			s, err := newServer(namespace, fmt.Sprintf("%s-%s", c.Parent().Name(), c.Name()), config, logger)
			cmd.CheckError(err)

			cmd.CheckError(s.run())
		},
	}

	command.Flags().Var(logLevelFlag, "log-level", fmt.Sprintf("the level at which to log. Valid values are %s.", strings.Join(logLevelFlag.AllowedValues(), ", ")))
	command.Flags().StringVar(&config.pluginDir, "plugin-dir", config.pluginDir, "directory containing Velero plugins")
	command.Flags().StringVar(&config.metricsAddress, "metrics-address", config.metricsAddress, "the address to expose prometheus metrics")
	command.Flags().DurationVar(&config.backupSyncPeriod, "backup-sync-period", config.backupSyncPeriod, "how often to ensure all Velero backups in object storage exist as Backup API objects in the cluster")
	command.Flags().DurationVar(&config.podVolumeOperationTimeout, "restic-timeout", config.podVolumeOperationTimeout, "how long backups/restores of pod volumes should be allowed to run before timing out")
	command.Flags().BoolVar(&config.restoreOnly, "restore-only", config.restoreOnly, "run in a mode where only restores are allowed; backups, schedules, and garbage-collection are all disabled")
	command.Flags().StringSliceVar(&config.restoreResourcePriorities, "restore-resource-priorities", config.restoreResourcePriorities, "desired order of resource restores; any resource not in the list will be restored alphabetically after the prioritized resources")
	command.Flags().StringVar(&config.defaultBackupLocation, "default-backup-storage-location", config.defaultBackupLocation, "name of the default backup storage location")
	command.Flags().Var(&volumeSnapshotLocations, "default-volume-snapshot-locations", "list of unique volume providers and default volume snapshot location (provider1:location-01,provider2:location-02,...)")
	command.Flags().Float32Var(&config.clientQPS, "client-qps", config.clientQPS, "maximum number of requests per second by the server to the Kubernetes API once the burst limit has been reached")
	command.Flags().IntVar(&config.clientBurst, "client-burst", config.clientBurst, "maximum number of requests by the server to the Kubernetes API in a short period of time")
	command.Flags().StringVar(&config.profilerAddress, "profiler-address", config.profilerAddress, "the address to expose the pprof profiler")
	command.Flags().DurationVar(&config.resourceTerminatingTimeout, "terminating-resource-timeout", config.resourceTerminatingTimeout, "how long to wait on persistent volumes and namespaces to terminate during a restore before timing out")

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
	veleroClient          clientset.Interface
	discoveryClient       discovery.DiscoveryInterface
	discoveryHelper       velerodiscovery.Helper
	dynamicClient         dynamic.Interface
	sharedInformerFactory informers.SharedInformerFactory
	ctx                   context.Context
	cancelFunc            context.CancelFunc
	logger                logrus.FieldLogger
	logLevel              logrus.Level
	pluginRegistry        plugin.Registry
	pluginManager         plugin.Manager
	resticManager         restic.RepositoryManager
	metrics               *metrics.ServerMetrics
	config                serverConfig
}

func newServer(namespace, baseName string, config serverConfig, logger *logrus.Logger) (*server, error) {
	clientConfig, err := client.Config("", "", baseName)
	if err != nil {
		return nil, err
	}
	if config.clientQPS < 0.0 {
		return nil, errors.New("client-qps must be positive")
	}
	clientConfig.QPS = config.clientQPS

	if config.clientBurst <= 0 {
		return nil, errors.New("client-burst must be positive")
	}
	clientConfig.Burst = config.clientBurst

	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	veleroClient, err := clientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	pluginRegistry := plugin.NewRegistry(config.pluginDir, logger, logger.Level)
	if err := pluginRegistry.DiscoverPlugins(); err != nil {
		return nil, err
	}
	pluginManager := plugin.NewManager(logger, logger.Level, pluginRegistry)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	ctx, cancelFunc := context.WithCancel(context.Background())

	s := &server{
		namespace:             namespace,
		metricsAddress:        config.metricsAddress,
		kubeClientConfig:      clientConfig,
		kubeClient:            kubeClient,
		veleroClient:          veleroClient,
		discoveryClient:       veleroClient.Discovery(),
		dynamicClient:         dynamicClient,
		sharedInformerFactory: informers.NewSharedInformerFactoryWithOptions(veleroClient, 0, informers.WithNamespace(namespace)),
		ctx:                   ctx,
		cancelFunc:            cancelFunc,
		logger:                logger,
		logLevel:              logger.Level,
		pluginRegistry:        pluginRegistry,
		pluginManager:         pluginManager,
		config:                config,
	}

	return s, nil
}

func (s *server) run() error {
	defer s.pluginManager.CleanupClients()

	signals.CancelOnShutdown(s.cancelFunc, s.logger)

	if s.config.profilerAddress != "" {
		go s.runProfiler()
	}

	// Since s.namespace, which specifies where backups/restores/schedules/etc. should live,
	// *could* be different from the namespace where the Velero server pod runs, check to make
	// sure it exists, and fail fast if it doesn't.
	if err := s.namespaceExists(s.namespace); err != nil {
		return err
	}

	if err := s.initDiscoveryHelper(); err != nil {
		return err
	}

	if err := s.veleroResourcesExist(); err != nil {
		return err
	}

	if err := s.validateBackupStorageLocations(); err != nil {
		return err
	}

	if _, err := s.veleroClient.VeleroV1().BackupStorageLocations(s.namespace).Get(s.config.defaultBackupLocation, metav1.GetOptions{}); err != nil {
		s.logger.WithError(errors.WithStack(err)).
			Warnf("A backup storage location named %s has been specified for the server to use by default, but no corresponding backup storage location exists. Backups with a location not matching the default will need to explicitly specify an existing location", s.config.defaultBackupLocation)
	}

	if err := s.initRestic(); err != nil {
		return err
	}

	if err := s.runControllers(s.config.defaultVolumeSnapshotLocations); err != nil {
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
	discoveryHelper, err := velerodiscovery.NewHelper(s.discoveryClient, s.logger)
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

// veleroResourcesExist checks for the existence of each Velero CRD via discovery
// and returns an error if any of them don't exist.
func (s *server) veleroResourcesExist() error {
	s.logger.Info("Checking existence of Velero custom resource definitions")

	var veleroGroupVersion *metav1.APIResourceList
	for _, gv := range s.discoveryHelper.Resources() {
		if gv.GroupVersion == api.SchemeGroupVersion.String() {
			veleroGroupVersion = gv
			break
		}
	}

	if veleroGroupVersion == nil {
		return errors.Errorf("Velero API group %s not found. Apply examples/common/00-prereqs.yaml to create it.", api.SchemeGroupVersion)
	}

	foundResources := sets.NewString()
	for _, resource := range veleroGroupVersion.APIResources {
		foundResources.Insert(resource.Kind)
	}

	var errs []error
	for kind := range api.CustomResources() {
		if foundResources.Has(kind) {
			s.logger.WithField("kind", kind).Debug("Found custom resource")
			continue
		}

		errs = append(errs, errors.Errorf("custom resource %s not found in Velero API group %s", kind, api.SchemeGroupVersion))
	}

	if len(errs) > 0 {
		errs = append(errs, errors.New("Velero custom resources not found - apply examples/common/00-prereqs.yaml to update the custom resource definitions"))
		return kubeerrs.NewAggregate(errs)
	}

	s.logger.Info("All Velero custom resource definitions exist")
	return nil
}

// validateBackupStorageLocations checks to ensure all backup storage locations exist
// and have a compatible layout, and returns an error if not.
func (s *server) validateBackupStorageLocations() error {
	s.logger.Info("Checking that all backup storage locations are valid")

	locations, err := s.veleroClient.VeleroV1().BackupStorageLocations(s.namespace).List(metav1.ListOptions{})
	if err != nil {
		return errors.WithStack(err)
	}

	var invalid []string
	for _, location := range locations.Items {
		backupStore, err := persistence.NewObjectBackupStore(&location, s.pluginManager, s.logger)
		if err != nil {
			invalid = append(invalid, errors.Wrapf(err, "error getting backup store for location %q", location.Name).Error())
			continue
		}

		if err := backupStore.IsValid(); err != nil {
			invalid = append(invalid, errors.Wrapf(err,
				"backup store for location %q is invalid (if upgrading from a pre-v0.10 version of Velero, please refer to https://heptio.github.io/velero/v0.10.0/storage-layout-reorg-v0.10 for instructions)",
				location.Name,
			).Error())
		}
	}

	if len(invalid) > 0 {
		return errors.Errorf("some backup storage locations are invalid: %s", strings.Join(invalid, "; "))
	}

	return nil
}

// - Namespaces go first because all namespaced resources depend on them.
// - Storage Classes are needed to create PVs and PVCs correctly.
// - PVs go before PVCs because PVCs depend on them.
// - PVCs go before pods or controllers so they can be mounted as volumes.
// - Secrets and config maps go before pods or controllers so they can be mounted
// 	 as volumes.
// - Service accounts go before pods or controllers so pods can use them.
// - Limit ranges go before pods or controllers so pods can use them.
// - Pods go before controllers so they can be explicitly restored and potentially
//	 have restic restores run before controllers adopt the pods.
// - Custom Resource Definitions come before Custom Resource so that they can be
//   restored with their corresponding CRD.
var defaultRestorePriorities = []string{
	"namespaces",
	"storageclasses",
	"persistentvolumes",
	"persistentvolumeclaims",
	"secrets",
	"configmaps",
	"serviceaccounts",
	"limitranges",
	"pods",
	"replicaset",
	"customresourcedefinitions",
}

func (s *server) initRestic() error {
	// warn if restic daemonset does not exist
	if _, err := s.kubeClient.AppsV1().DaemonSets(s.namespace).Get(restic.DaemonSet, metav1.GetOptions{}); apierrors.IsNotFound(err) {
		s.logger.Warn("Velero restic daemonset not found; restic backups/restores will not work until it's created")
	} else if err != nil {
		s.logger.WithError(errors.WithStack(err)).Warn("Error checking for existence of velero restic daemonset")
	}

	// ensure the repo key secret is set up
	if err := restic.EnsureCommonRepositoryKey(s.kubeClient.CoreV1(), s.namespace); err != nil {
		return err
	}

	// use a stand-alone secrets informer so we can filter to only the restic credentials
	// secret(s) within the velero namespace
	//
	// note: using an informer to access the single secret for all velero-managed
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
		s.veleroClient,
		secretsInformer,
		s.sharedInformerFactory.Velero().V1().ResticRepositories(),
		s.veleroClient.VeleroV1(),
		s.sharedInformerFactory.Velero().V1().BackupStorageLocations(),
		s.logger,
	)
	if err != nil {
		return err
	}
	s.resticManager = res

	return nil
}

func (s *server) runControllers(defaultVolumeSnapshotLocations map[string]string) error {
	s.logger.Info("Starting controllers")

	ctx := s.ctx
	var wg sync.WaitGroup

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
	// Initialize manual backup metrics
	s.metrics.InitSchedule("")

	newPluginManager := func(logger logrus.FieldLogger) plugin.Manager {
		return plugin.NewManager(logger, s.logLevel, s.pluginRegistry)
	}

	backupSyncController := controller.NewBackupSyncController(
		s.veleroClient.VeleroV1(),
		s.veleroClient.VeleroV1(),
		s.sharedInformerFactory.Velero().V1().Backups(),
		s.sharedInformerFactory.Velero().V1().BackupStorageLocations(),
		s.config.backupSyncPeriod,
		s.namespace,
		s.config.defaultBackupLocation,
		newPluginManager,
		s.logger,
	)
	wg.Add(1)
	go func() {
		backupSyncController.Run(ctx, 1)
		wg.Done()
	}()

	if s.config.restoreOnly {
		s.logger.Info("Restore only mode - not starting the backup, schedule, delete-backup, or GC controllers")
	} else {
		backupTracker := controller.NewBackupTracker()

		backupper, err := backup.NewKubernetesBackupper(
			s.discoveryHelper,
			client.NewDynamicFactory(s.dynamicClient),
			podexec.NewPodCommandExecutor(s.kubeClientConfig, s.kubeClient.CoreV1().RESTClient()),
			s.resticManager,
			s.config.podVolumeOperationTimeout,
		)
		cmd.CheckError(err)

		backupController := controller.NewBackupController(
			s.sharedInformerFactory.Velero().V1().Backups(),
			s.veleroClient.VeleroV1(),
			backupper,
			s.logger,
			s.logLevel,
			newPluginManager,
			backupTracker,
			s.sharedInformerFactory.Velero().V1().BackupStorageLocations(),
			s.config.defaultBackupLocation,
			s.sharedInformerFactory.Velero().V1().VolumeSnapshotLocations(),
			defaultVolumeSnapshotLocations,
			s.metrics,
		)
		wg.Add(1)
		go func() {
			backupController.Run(ctx, 1)
			wg.Done()
		}()

		scheduleController := controller.NewScheduleController(
			s.namespace,
			s.veleroClient.VeleroV1(),
			s.veleroClient.VeleroV1(),
			s.sharedInformerFactory.Velero().V1().Schedules(),
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
			s.sharedInformerFactory.Velero().V1().Backups(),
			s.sharedInformerFactory.Velero().V1().DeleteBackupRequests(),
			s.veleroClient.VeleroV1(),
		)
		wg.Add(1)
		go func() {
			gcController.Run(ctx, 1)
			wg.Done()
		}()

		backupDeletionController := controller.NewBackupDeletionController(
			s.logger,
			s.sharedInformerFactory.Velero().V1().DeleteBackupRequests(),
			s.veleroClient.VeleroV1(), // deleteBackupRequestClient
			s.veleroClient.VeleroV1(), // backupClient
			s.sharedInformerFactory.Velero().V1().Restores(),
			s.veleroClient.VeleroV1(), // restoreClient
			backupTracker,
			s.resticManager,
			s.sharedInformerFactory.Velero().V1().PodVolumeBackups(),
			s.sharedInformerFactory.Velero().V1().BackupStorageLocations(),
			s.sharedInformerFactory.Velero().V1().VolumeSnapshotLocations(),
			newPluginManager,
		)
		wg.Add(1)
		go func() {
			backupDeletionController.Run(ctx, 1)
			wg.Done()
		}()
	}

	restorer, err := restore.NewKubernetesRestorer(
		s.discoveryHelper,
		client.NewDynamicFactory(s.dynamicClient),
		s.config.restoreResourcePriorities,
		s.kubeClient.CoreV1().Namespaces(),
		s.resticManager,
		s.config.podVolumeOperationTimeout,
		s.config.resourceTerminatingTimeout,
		s.logger,
	)
	cmd.CheckError(err)

	restoreController := controller.NewRestoreController(
		s.namespace,
		s.sharedInformerFactory.Velero().V1().Restores(),
		s.veleroClient.VeleroV1(),
		s.veleroClient.VeleroV1(),
		restorer,
		s.sharedInformerFactory.Velero().V1().Backups(),
		s.sharedInformerFactory.Velero().V1().BackupStorageLocations(),
		s.sharedInformerFactory.Velero().V1().VolumeSnapshotLocations(),
		s.logger,
		s.logLevel,
		newPluginManager,
		s.config.defaultBackupLocation,
		s.metrics,
	)

	wg.Add(1)
	go func() {
		restoreController.Run(ctx, 1)
		wg.Done()
	}()

	downloadRequestController := controller.NewDownloadRequestController(
		s.veleroClient.VeleroV1(),
		s.sharedInformerFactory.Velero().V1().DownloadRequests(),
		s.sharedInformerFactory.Velero().V1().Restores(),
		s.sharedInformerFactory.Velero().V1().BackupStorageLocations(),
		s.sharedInformerFactory.Velero().V1().Backups(),
		newPluginManager,
		s.logger,
	)
	wg.Add(1)
	go func() {
		downloadRequestController.Run(ctx, 1)
		wg.Done()
	}()

	resticRepoController := controller.NewResticRepositoryController(
		s.logger,
		s.sharedInformerFactory.Velero().V1().ResticRepositories(),
		s.veleroClient.VeleroV1(),
		s.sharedInformerFactory.Velero().V1().BackupStorageLocations(),
		s.resticManager,
	)
	wg.Add(1)
	go func() {
		// TODO only having a single worker may be an issue since maintenance
		// can take a long time.
		resticRepoController.Run(ctx, 1)
		wg.Done()
	}()

	serverStatusRequestController := controller.NewServerStatusRequestController(
		s.logger,
		s.veleroClient.VeleroV1(),
		s.sharedInformerFactory.Velero().V1().ServerStatusRequests(),
	)
	wg.Add(1)
	go func() {
		serverStatusRequestController.Run(ctx, 1)
		wg.Done()
	}()

	// SHARED INFORMERS HAVE TO BE STARTED AFTER ALL CONTROLLERS
	go s.sharedInformerFactory.Start(ctx.Done())

	// TODO(1.0): remove
	cache.WaitForCacheSync(ctx.Done(), s.sharedInformerFactory.Velero().V1().Backups().Informer().HasSynced)
	s.removeDeprecatedGCFinalizer()

	s.logger.Info("Server started successfully")

	<-ctx.Done()

	s.logger.Info("Waiting for all controllers to shut down gracefully")
	wg.Wait()

	return nil
}

func (s *server) runProfiler() {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	if err := http.ListenAndServe(s.config.profilerAddress, mux); err != nil {
		s.logger.WithError(errors.WithStack(err)).Error("error running profiler http server")
	}
}

// TODO(1.0): remove
func (s *server) removeDeprecatedGCFinalizer() {
	const gcFinalizer = "gc.ark.heptio.com"

	backups, err := s.sharedInformerFactory.Velero().V1().Backups().Lister().List(labels.Everything())
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

		_, err = s.veleroClient.VeleroV1().Backups(backup.Namespace).Patch(backup.Name, types.MergePatchType, patchBytes)
		if err != nil {
			log.WithError(errors.WithStack(err)).Error("error marshaling finalizers patch")
		}
	}
}
