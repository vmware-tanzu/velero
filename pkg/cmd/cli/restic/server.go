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
package restic

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/vmware-tanzu/velero/internal/util/managercontroller"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/metrics"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	kubeinformers "k8s.io/client-go/informers"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/vmware-tanzu/velero/pkg/buildinfo"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/util/signals"
	"github.com/vmware-tanzu/velero/pkg/controller"
	clientset "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned"
	informers "github.com/vmware-tanzu/velero/pkg/generated/informers/externalversions"
	"github.com/vmware-tanzu/velero/pkg/restic"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
	"github.com/vmware-tanzu/velero/pkg/util/logging"

	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	scheme = runtime.NewScheme()
)

const (
	// the port where prometheus metrics are exposed
	defaultMetricsAddress = ":8085"
)

func NewServerCommand(f client.Factory) *cobra.Command {
	logLevelFlag := logging.LogLevelFlag(logrus.InfoLevel)
	formatFlag := logging.NewFormatFlag()

	command := &cobra.Command{
		Use:    "server",
		Short:  "Run the velero restic server",
		Long:   "Run the velero restic server",
		Hidden: true,
		Run: func(c *cobra.Command, args []string) {
			logLevel := logLevelFlag.Parse()
			logrus.Infof("Setting log-level to %s", strings.ToUpper(logLevel.String()))

			logger := logging.DefaultLogger(logLevel, formatFlag.Parse())
			logger.Infof("Starting Velero restic server %s (%s)", buildinfo.Version, buildinfo.FormattedGitSHA())

			f.SetBasename(fmt.Sprintf("%s-%s", c.Parent().Name(), c.Name()))
			s, err := newResticServer(logger, f, defaultMetricsAddress)
			cmd.CheckError(err)

			s.run()
		},
	}

	command.Flags().Var(logLevelFlag, "log-level", fmt.Sprintf("The level at which to log. Valid values are %s.", strings.Join(logLevelFlag.AllowedValues(), ", ")))
	command.Flags().Var(formatFlag, "log-format", fmt.Sprintf("The format for log output. Valid values are %s.", strings.Join(formatFlag.AllowedValues(), ", ")))

	return command
}

type resticServer struct {
	kubeClient            kubernetes.Interface
	veleroClient          clientset.Interface
	veleroInformerFactory informers.SharedInformerFactory
	kubeInformerFactory   kubeinformers.SharedInformerFactory
	podInformer           cache.SharedIndexInformer
	secretInformer        cache.SharedIndexInformer
	logger                logrus.FieldLogger
	ctx                   context.Context
	cancelFunc            context.CancelFunc
	fileSystem            filesystem.Interface
	mgr                   manager.Manager
	metrics               *metrics.ServerMetrics
	metricsAddress        string
}

func newResticServer(logger logrus.FieldLogger, factory client.Factory, metricAddress string) (*resticServer, error) {

	kubeClient, err := factory.KubeClient()
	if err != nil {
		return nil, err
	}

	veleroClient, err := factory.Client()
	if err != nil {
		return nil, err
	}

	// use a stand-alone pod informer because we want to use a field selector to
	// filter to only pods scheduled on this node.
	podInformer := corev1informers.NewFilteredPodInformer(
		kubeClient,
		metav1.NamespaceAll,
		0,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		func(opts *metav1.ListOptions) {
			opts.FieldSelector = fmt.Sprintf("spec.nodeName=%s", os.Getenv("NODE_NAME"))
		},
	)

	// use a stand-alone secrets informer so we can filter to only the restic credentials
	// secret(s) within the velero namespace
	//
	// note: using an informer to access the single secret for all velero-managed
	// restic repositories is overkill for now, but will be useful when we move
	// to fully-encrypted backups and have unique keys per repository.
	secretInformer := corev1informers.NewFilteredSecretInformer(
		kubeClient,
		factory.Namespace(),
		0,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		func(opts *metav1.ListOptions) {
			opts.FieldSelector = fmt.Sprintf("metadata.name=%s", restic.CredentialsSecretName)
		},
	)

	ctx, cancelFunc := context.WithCancel(context.Background())

	clientConfig, err := factory.ClientConfig()
	if err != nil {
		return nil, err
	}

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	velerov1api.AddToScheme(scheme)
	mgr, err := ctrl.NewManager(clientConfig, ctrl.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, err
	}

	s := &resticServer{
		kubeClient:            kubeClient,
		veleroClient:          veleroClient,
		veleroInformerFactory: informers.NewFilteredSharedInformerFactory(veleroClient, 0, factory.Namespace(), nil),
		kubeInformerFactory:   kubeinformers.NewSharedInformerFactory(kubeClient, 0),
		podInformer:           podInformer,
		secretInformer:        secretInformer,
		logger:                logger,
		ctx:                   ctx,
		cancelFunc:            cancelFunc,
		fileSystem:            filesystem.NewFileSystem(),
		mgr:                   mgr,
		metricsAddress:        metricAddress,
	}

	if err := s.validatePodVolumesHostPath(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *resticServer) run() {
	signals.CancelOnShutdown(s.cancelFunc, s.logger)

	go func() {
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", promhttp.Handler())
		s.logger.Infof("Starting metric server for restic at address [%s]", s.metricsAddress)
		if err := http.ListenAndServe(s.metricsAddress, metricsMux); err != nil {
			s.logger.Fatalf("Failed to start metric server for restic at [%s]: %v", s.metricsAddress, err)
		}
	}()
	s.metrics = metrics.NewResticServerMetrics()
	s.metrics.RegisterAllMetrics()
	s.metrics.InitResticMetricsForNode(os.Getenv("NODE_NAME"))

	s.logger.Info("Starting controllers")

	backupController := controller.NewPodVolumeBackupController(
		s.logger,
		s.veleroInformerFactory.Velero().V1().PodVolumeBackups(),
		s.veleroClient.VeleroV1(),
		s.podInformer,
		s.secretInformer,
		s.kubeInformerFactory.Core().V1().PersistentVolumeClaims(),
		s.kubeInformerFactory.Core().V1().PersistentVolumes(),
		s.metrics,
		s.mgr.GetClient(),
		os.Getenv("NODE_NAME"),
	)

	restoreController := controller.NewPodVolumeRestoreController(
		s.logger,
		s.veleroInformerFactory.Velero().V1().PodVolumeRestores(),
		s.veleroClient.VeleroV1(),
		s.podInformer,
		s.secretInformer,
		s.kubeInformerFactory.Core().V1().PersistentVolumeClaims(),
		s.kubeInformerFactory.Core().V1().PersistentVolumes(),
		s.mgr.GetClient(),
		os.Getenv("NODE_NAME"),
	)

	go s.veleroInformerFactory.Start(s.ctx.Done())
	go s.kubeInformerFactory.Start(s.ctx.Done())
	go s.podInformer.Run(s.ctx.Done())
	go s.secretInformer.Run(s.ctx.Done())

	// TODO(2.0): presuming all controllers and resources are converted to runtime-controller
	// by v2.0, the block from this line and including the `s.mgr.Start() will be
	// deprecated, since the manager auto-starts all the caches. Until then, we need to start the
	// cache for them manually.

	// Adding the controllers to the manager will register them as a (runtime-controller) runnable,
	// so the manager will ensure the cache is started and ready before all controller are started
	s.mgr.Add(managercontroller.Runnable(backupController, 1))
	s.mgr.Add(managercontroller.Runnable(restoreController, 1))

	s.logger.Info("Controllers starting...")

	if err := s.mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		s.logger.Fatal("Problem starting manager", err)
	}
}

// validatePodVolumesHostPath validates that the pod volumes path contains a
// directory for each Pod running on this node
func (s *resticServer) validatePodVolumesHostPath() error {
	files, err := s.fileSystem.ReadDir("/host_pods/")
	if err != nil {
		return errors.Wrap(err, "could not read pod volumes host path")
	}

	// create a map of directory names inside the pod volumes path
	dirs := sets.NewString()
	for _, f := range files {
		if f.IsDir() {
			dirs.Insert(f.Name())
		}
	}

	pods, err := s.kubeClient.CoreV1().Pods("").List(s.ctx, metav1.ListOptions{FieldSelector: fmt.Sprintf("spec.nodeName=%s,status.phase=Running", os.Getenv("NODE_NAME"))})
	if err != nil {
		return errors.WithStack(err)
	}

	valid := true
	for _, pod := range pods.Items {
		dirName := string(pod.GetUID())

		// if the pod is a mirror pod, the directory name is the hash value of the
		// mirror pod annotation
		if hash, ok := pod.GetAnnotations()[v1.MirrorPodAnnotationKey]; ok {
			dirName = hash
		}

		if !dirs.Has(dirName) {
			valid = false
			s.logger.WithFields(logrus.Fields{
				"pod":  fmt.Sprintf("%s/%s", pod.GetNamespace(), pod.GetName()),
				"path": "/host_pods/" + dirName,
			}).Debug("could not find volumes for pod in host path")
		}
	}

	if !valid {
		return errors.New("unexpected directory structure for host-pods volume, ensure that the host-pods volume corresponds to the pods subdirectory of the kubelet root directory")
	}

	return nil
}
