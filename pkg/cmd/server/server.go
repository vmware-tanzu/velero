/*
Copyright 2017, 2019 the Velero contributors.

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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeerrs "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/backup"
	"github.com/vmware-tanzu/velero/pkg/buildinfo"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/util/flag"
	"github.com/vmware-tanzu/velero/pkg/cmd/util/signals"
	"github.com/vmware-tanzu/velero/pkg/controller"
	velerodiscovery "github.com/vmware-tanzu/velero/pkg/discovery"
	"github.com/vmware-tanzu/velero/pkg/features"
	clientset "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned"
	informers "github.com/vmware-tanzu/velero/pkg/generated/informers/externalversions"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	"github.com/vmware-tanzu/velero/pkg/podexec"
	"github.com/vmware-tanzu/velero/pkg/restic"
	"github.com/vmware-tanzu/velero/pkg/restore"
	"github.com/vmware-tanzu/velero/pkg/util/logging"
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

	// keys used to map out available controllers with disable-controllers flag
	BackupControllerKey              = "backup"
	BackupSyncControllerKey          = "backup-sync"
	ScheduleControllerKey            = "schedule"
	GcControllerKey                  = "gc"
	BackupDeletionControllerKey      = "backup-deletion"
	RestoreControllerKey             = "restore"
	DownloadRequestControllerKey     = "download-request"
	ResticRepoControllerKey          = "restic-repo"
	ServerStatusRequestControllerKey = "server-status-request"

	defaultControllerWorkers = 1
	// the default TTL for a backup
	defaultBackupTTL = 30 * 24 * time.Hour
)

// list of available controllers for input validation
var disableControllerList = []string{
	BackupControllerKey,
	BackupSyncControllerKey,
	ScheduleControllerKey,
	GcControllerKey,
	BackupDeletionControllerKey,
	RestoreControllerKey,
	DownloadRequestControllerKey,
	ResticRepoControllerKey,
	ServerStatusRequestControllerKey,
}

type serverConfig struct {
	pluginDir, metricsAddress, defaultBackupLocation                        string
	backupSyncPeriod, podVolumeOperationTimeout, resourceTerminatingTimeout time.Duration
	defaultBackupTTL                                                        time.Duration
	restoreResourcePriorities                                               []string
	defaultVolumeSnapshotLocations                                          map[string]string
	restoreOnly                                                             bool
	disabledControllers                                                     []string
	clientQPS                                                               float32
	clientBurst                                                             int
	profilerAddress                                                         string
	formatFlag                                                              *logging.FormatFlag
	defaultResticMaintenanceFrequency                                       time.Duration
}

type controllerRunInfo struct {
	controller controller.Interface
	numWorkers int
}

func NewCommand(f client.Factory) *cobra.Command {
	var (
		volumeSnapshotLocations = flag.NewMap().WithKeyValueDelimiter(":")
		logLevelFlag            = logging.LogLevelFlag(logrus.InfoLevel)
		config                  = serverConfig{
			pluginDir:                         "/plugins",
			metricsAddress:                    defaultMetricsAddress,
			defaultBackupLocation:             "default",
			defaultVolumeSnapshotLocations:    make(map[string]string),
			backupSyncPeriod:                  defaultBackupSyncPeriod,
			defaultBackupTTL:                  defaultBackupTTL,
			podVolumeOperationTimeout:         defaultPodVolumeOperationTimeout,
			restoreResourcePriorities:         defaultRestorePriorities,
			clientQPS:                         defaultClientQPS,
			clientBurst:                       defaultClientBurst,
			profilerAddress:                   defaultProfilerAddress,
			resourceTerminatingTimeout:        defaultResourceTerminatingTimeout,
			formatFlag:                        logging.NewFormatFlag(),
			defaultResticMaintenanceFrequency: restic.DefaultMaintenanceFrequency,
		}
	)

	var command = &cobra.Command{
		Use:    "server",
		Short:  "Run the velero server",
		Long:   "Run the velero server",
		Hidden: true,
		Run: func(c *cobra.Command, args []string) {
			// go-plugin uses log.Println to log when it's waiting for all plugin processes to complete so we need to
			// set its output to stdout.
			log.SetOutput(os.Stdout)

			logLevel := logLevelFlag.Parse()
			format := config.formatFlag.Parse()

			// Make sure we log to stdout so cloud log dashboards don't show this as an error.
			logrus.SetOutput(os.Stdout)

			// Velero's DefaultLogger logs to stdout, so all is good there.
			logger := logging.DefaultLogger(logLevel, format)

			logger.Infof("setting log-level to %s", strings.ToUpper(logLevel.String()))

			logger.Infof("Starting Velero server %s (%s)", buildinfo.Version, buildinfo.FormattedGitSHA())
			if len(features.All()) > 0 {
				logger.Infof("%d feature flags enabled %s", len(features.All()), features.All())
			} else {
				logger.Info("No feature flags enabled")
			}

			if volumeSnapshotLocations.Data() != nil {
				config.defaultVolumeSnapshotLocations = volumeSnapshotLocations.Data()
			}

			f.SetBasename(fmt.Sprintf("%s-%s", c.Parent().Name(), c.Name()))

			s, err := newServer(f, config, logger)
			cmd.CheckError(err)

			cmd.CheckError(s.run())
		},
	}

	command.Flags().Var(logLevelFlag, "log-level", fmt.Sprintf("the level at which to log. Valid values are %s.", strings.Join(logLevelFlag.AllowedValues(), ", ")))
	command.Flags().Var(config.formatFlag, "log-format", fmt.Sprintf("the format for log output. Valid values are %s.", strings.Join(config.formatFlag.AllowedValues(), ", ")))
	command.Flags().StringVar(&config.pluginDir, "plugin-dir", config.pluginDir, "directory containing Velero plugins")
	command.Flags().StringVar(&config.metricsAddress, "metrics-address", config.metricsAddress, "the address to expose prometheus metrics")
	command.Flags().DurationVar(&config.backupSyncPeriod, "backup-sync-period", config.backupSyncPeriod, "how often to ensure all Velero backups in object storage exist as Backup API objects in the cluster. This is the default sync period if none is explicitly specified for a backup storage location.")
	command.Flags().DurationVar(&config.podVolumeOperationTimeout, "restic-timeout", config.podVolumeOperationTimeout, "how long backups/restores of pod volumes should be allowed to run before timing out")
	command.Flags().BoolVar(&config.restoreOnly, "restore-only", config.restoreOnly, "run in a mode where only restores are allowed; backups, schedules, and garbage-collection are all disabled. DEPRECATED: this flag will be removed in v2.0. Use read-only backup storage locations instead.")
	command.Flags().StringSliceVar(&config.disabledControllers, "disable-controllers", config.disabledControllers, fmt.Sprintf("list of controllers to disable on startup. Valid values are %s", strings.Join(disableControllerList, ",")))
	command.Flags().StringSliceVar(&config.restoreResourcePriorities, "restore-resource-priorities", config.restoreResourcePriorities, "desired order of resource restores; any resource not in the list will be restored alphabetically after the prioritized resources")
	command.Flags().StringVar(&config.defaultBackupLocation, "default-backup-storage-location", config.defaultBackupLocation, "name of the default backup storage location")
	command.Flags().Var(&volumeSnapshotLocations, "default-volume-snapshot-locations", "list of unique volume providers and default volume snapshot location (provider1:location-01,provider2:location-02,...)")
	command.Flags().Float32Var(&config.clientQPS, "client-qps", config.clientQPS, "maximum number of requests per second by the server to the Kubernetes API once the burst limit has been reached")
	command.Flags().IntVar(&config.clientBurst, "client-burst", config.clientBurst, "maximum number of requests by the server to the Kubernetes API in a short period of time")
	command.Flags().StringVar(&config.profilerAddress, "profiler-address", config.profilerAddress, "the address to expose the pprof profiler")
	command.Flags().DurationVar(&config.resourceTerminatingTimeout, "terminating-resource-timeout", config.resourceTerminatingTimeout, "how long to wait on persistent volumes and namespaces to terminate during a restore before timing out")
	command.Flags().DurationVar(&config.defaultBackupTTL, "default-backup-ttl", config.defaultBackupTTL, "how long to wait by default before backups can be garbage collected")
	command.Flags().DurationVar(&config.defaultResticMaintenanceFrequency, "default-restic-prune-frequency", config.defaultResticMaintenanceFrequency, "how often 'restic prune' is run for restic repositories by default")

	return command
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
	pluginRegistry        clientmgmt.Registry
	resticManager         restic.RepositoryManager
	metrics               *metrics.ServerMetrics
	config                serverConfig
}

func newServer(f client.Factory, config serverConfig, logger *logrus.Logger) (*server, error) {
	if config.clientQPS < 0.0 {
		return nil, errors.New("client-qps must be positive")
	}
	f.SetClientQPS(config.clientQPS)

	if config.clientBurst <= 0 {
		return nil, errors.New("client-burst must be positive")
	}
	f.SetClientBurst(config.clientBurst)

	kubeClient, err := f.KubeClient()
	if err != nil {
		return nil, err
	}

	veleroClient, err := f.Client()
	if err != nil {
		return nil, err
	}

	dynamicClient, err := f.DynamicClient()
	if err != nil {
		return nil, err
	}

	pluginRegistry := clientmgmt.NewRegistry(config.pluginDir, logger, logger.Level)
	if err := pluginRegistry.DiscoverPlugins(); err != nil {
		return nil, err
	}

	ctx, cancelFunc := context.WithCancel(context.Background())

	clientConfig, err := f.ClientConfig()
	if err != nil {
		return nil, err
	}

	s := &server{
		namespace:             f.Namespace(),
		metricsAddress:        config.metricsAddress,
		kubeClientConfig:      clientConfig,
		kubeClient:            kubeClient,
		veleroClient:          veleroClient,
		discoveryClient:       veleroClient.Discovery(),
		dynamicClient:         dynamicClient,
		sharedInformerFactory: informers.NewSharedInformerFactoryWithOptions(veleroClient, 0, informers.WithNamespace(f.Namespace())),
		ctx:                   ctx,
		cancelFunc:            cancelFunc,
		logger:                logger,
		logLevel:              logger.Level,
		pluginRegistry:        pluginRegistry,
		config:                config,
	}

	return s, nil
}

func (s *server) run() error {
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

	pluginManager := clientmgmt.NewManager(s.logger, s.logLevel, s.pluginRegistry)
	defer pluginManager.CleanupClients()

	locations, err := s.veleroClient.VeleroV1().BackupStorageLocations(s.namespace).List(metav1.ListOptions{})
	if err != nil {
		return errors.WithStack(err)
	}

	var invalid []string
	for _, location := range locations.Items {
		backupStore, err := persistence.NewObjectBackupStore(&location, pluginManager, s.logger)
		if err != nil {
			invalid = append(invalid, errors.Wrapf(err, "error getting backup store for location %q", location.Name).Error())
			continue
		}

		if err := backupStore.IsValid(); err != nil {
			invalid = append(invalid, errors.Wrapf(err, "backup store for location %q is invalid", location.Name).Error())
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
// - Replica sets go before deployments/other controllers so they can be explicitly
//	 restored and be adopted by controllers.
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
	// we fully qualify replicasets.apps because prior to Kubernetes 1.16, replicasets also
	// existed in the extensions API group, but we back up replicasets from "apps" so we want
	// to ensure that we prioritize restoring from "apps" too, since this is how they're stored
	// in the backup.
	"replicasets.apps",
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
		s.kubeClient.CoreV1(),
		s.kubeClient.CoreV1(),
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

	newPluginManager := func(logger logrus.FieldLogger) clientmgmt.Manager {
		return clientmgmt.NewManager(logger, s.logLevel, s.pluginRegistry)
	}

	backupSyncControllerRunInfo := func() controllerRunInfo {
		backupSyncContoller := controller.NewBackupSyncController(
			s.veleroClient.VeleroV1(),
			s.veleroClient.VeleroV1(),
			s.veleroClient.VeleroV1(),
			s.sharedInformerFactory.Velero().V1().Backups(),
			s.sharedInformerFactory.Velero().V1().BackupStorageLocations(),
			s.sharedInformerFactory.Velero().V1().PodVolumeBackups(),
			s.config.backupSyncPeriod,
			s.namespace,
			s.config.defaultBackupLocation,
			newPluginManager,
			s.logger,
		)

		return controllerRunInfo{
			controller: backupSyncContoller,
			numWorkers: defaultControllerWorkers,
		}
	}

	backupTracker := controller.NewBackupTracker()

	backupControllerRunInfo := func() controllerRunInfo {
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
			s.config.defaultBackupTTL,
			s.sharedInformerFactory.Velero().V1().VolumeSnapshotLocations(),
			defaultVolumeSnapshotLocations,
			s.metrics,
			s.config.formatFlag.Parse(),
		)

		return controllerRunInfo{
			controller: backupController,
			numWorkers: defaultControllerWorkers,
		}
	}

	scheduleControllerRunInfo := func() controllerRunInfo {
		scheduleController := controller.NewScheduleController(
			s.namespace,
			s.veleroClient.VeleroV1(),
			s.veleroClient.VeleroV1(),
			s.sharedInformerFactory.Velero().V1().Schedules(),
			s.logger,
			s.metrics,
		)

		return controllerRunInfo{
			controller: scheduleController,
			numWorkers: defaultControllerWorkers,
		}
	}

	gcControllerRunInfo := func() controllerRunInfo {
		gcController := controller.NewGCController(
			s.logger,
			s.sharedInformerFactory.Velero().V1().Backups(),
			s.sharedInformerFactory.Velero().V1().DeleteBackupRequests(),
			s.veleroClient.VeleroV1(),
			s.sharedInformerFactory.Velero().V1().BackupStorageLocations(),
		)

		return controllerRunInfo{
			controller: gcController,
			numWorkers: defaultControllerWorkers,
		}
	}

	deletionControllerRunInfo := func() controllerRunInfo {
		deletionController := controller.NewBackupDeletionController(
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
			s.metrics,
		)

		return controllerRunInfo{
			controller: deletionController,
			numWorkers: defaultControllerWorkers,
		}
	}

	restoreControllerRunInfo := func() controllerRunInfo {

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
			s.config.formatFlag.Parse(),
		)

		return controllerRunInfo{
			controller: restoreController,
			numWorkers: defaultControllerWorkers,
		}
	}

	resticRepoControllerRunInfo := func() controllerRunInfo {
		resticRepoController := controller.NewResticRepositoryController(
			s.logger,
			s.sharedInformerFactory.Velero().V1().ResticRepositories(),
			s.veleroClient.VeleroV1(),
			s.sharedInformerFactory.Velero().V1().BackupStorageLocations(),
			s.resticManager,
			s.config.defaultResticMaintenanceFrequency,
		)

		return controllerRunInfo{
			controller: resticRepoController,
			numWorkers: defaultControllerWorkers,
		}
	}

	downloadrequestControllerRunInfo := func() controllerRunInfo {
		downloadRequestController := controller.NewDownloadRequestController(
			s.veleroClient.VeleroV1(),
			s.sharedInformerFactory.Velero().V1().DownloadRequests(),
			s.sharedInformerFactory.Velero().V1().Restores(),
			s.sharedInformerFactory.Velero().V1().BackupStorageLocations(),
			s.sharedInformerFactory.Velero().V1().Backups(),
			newPluginManager,
			s.logger,
		)

		return controllerRunInfo{
			controller: downloadRequestController,
			numWorkers: defaultControllerWorkers,
		}
	}

	serverStatusRequestControllerRunInfo := func() controllerRunInfo {
		serverStatusRequestController := controller.NewServerStatusRequestController(
			s.logger,
			s.veleroClient.VeleroV1(),
			s.sharedInformerFactory.Velero().V1().ServerStatusRequests(),
			s.pluginRegistry,
		)

		return controllerRunInfo{
			controller: serverStatusRequestController,
			numWorkers: defaultControllerWorkers,
		}
	}

	enabledControllers := map[string]func() controllerRunInfo{
		BackupSyncControllerKey:          backupSyncControllerRunInfo,
		BackupControllerKey:              backupControllerRunInfo,
		ScheduleControllerKey:            scheduleControllerRunInfo,
		GcControllerKey:                  gcControllerRunInfo,
		BackupDeletionControllerKey:      deletionControllerRunInfo,
		RestoreControllerKey:             restoreControllerRunInfo,
		ResticRepoControllerKey:          resticRepoControllerRunInfo,
		DownloadRequestControllerKey:     downloadrequestControllerRunInfo,
		ServerStatusRequestControllerKey: serverStatusRequestControllerRunInfo,
	}

	if s.config.restoreOnly {
		s.logger.Info("Restore only mode - not starting the backup, schedule, delete-backup, or GC controllers")
		s.config.disabledControllers = append(s.config.disabledControllers,
			BackupControllerKey,
			ScheduleControllerKey,
			GcControllerKey,
			BackupDeletionControllerKey,
		)
	}

	// remove disabled controllers
	for _, controllerName := range s.config.disabledControllers {
		if _, ok := enabledControllers[controllerName]; ok {
			s.logger.Infof("Disabling controller: %s", controllerName)
			delete(enabledControllers, controllerName)
		} else {
			s.logger.Fatalf("Invalid value for --disable-controllers flag provided: %s. Valid values are: %s", controllerName, strings.Join(disableControllerList, ","))
		}
	}

	for _, newController := range enabledControllers {
		controllerRunInfo := newController()
		wg.Add(1)
		go func() {
			controllerRunInfo.controller.Run(ctx, controllerRunInfo.numWorkers)
			wg.Done()
		}()
	}

	// SHARED INFORMERS HAVE TO BE STARTED AFTER ALL CONTROLLERS
	go s.sharedInformerFactory.Start(ctx.Done())

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
