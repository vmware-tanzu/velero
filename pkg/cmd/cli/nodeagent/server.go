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

package nodeagent

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	storagev1api "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	clocks "k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/vmware-tanzu/velero/internal/credentials"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/buildinfo"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/util/signals"
	"github.com/vmware-tanzu/velero/pkg/controller"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
	"github.com/vmware-tanzu/velero/pkg/util/logging"
)

var (
	scheme = runtime.NewScheme()
)

const (
	// the port where prometheus metrics are exposed
	defaultMetricsAddress = ":8085"

	// defaultCredentialsDirectory is the path on disk where credential
	// files will be written to
	defaultCredentialsDirectory = "/tmp/credentials"
)

func NewServerCommand(f client.Factory) *cobra.Command {
	logLevelFlag := logging.LogLevelFlag(logrus.InfoLevel)
	formatFlag := logging.NewFormatFlag()

	command := &cobra.Command{
		Use:    "server",
		Short:  "Run the velero node-agent server",
		Long:   "Run the velero node-agent server",
		Hidden: true,
		Run: func(c *cobra.Command, args []string) {
			logLevel := logLevelFlag.Parse()
			logrus.Infof("Setting log-level to %s", strings.ToUpper(logLevel.String()))

			logger := logging.DefaultLogger(logLevel, formatFlag.Parse())
			logger.Infof("Starting Velero node-agent server %s (%s)", buildinfo.Version, buildinfo.FormattedGitSHA())

			f.SetBasename(fmt.Sprintf("%s-%s", c.Parent().Name(), c.Name()))
			s, err := newNodeAgentServer(logger, f, defaultMetricsAddress)
			cmd.CheckError(err)

			s.run()
		},
	}

	command.Flags().Var(logLevelFlag, "log-level", fmt.Sprintf("The level at which to log. Valid values are %s.", strings.Join(logLevelFlag.AllowedValues(), ", ")))
	command.Flags().Var(formatFlag, "log-format", fmt.Sprintf("The format for log output. Valid values are %s.", strings.Join(formatFlag.AllowedValues(), ", ")))

	return command
}

type nodeAgentServer struct {
	logger         logrus.FieldLogger
	ctx            context.Context
	cancelFunc     context.CancelFunc
	fileSystem     filesystem.Interface
	mgr            manager.Manager
	metrics        *metrics.ServerMetrics
	metricsAddress string
	namespace      string
	nodeName       string
}

func newNodeAgentServer(logger logrus.FieldLogger, factory client.Factory, metricAddress string) (*nodeAgentServer, error) {
	ctx, cancelFunc := context.WithCancel(context.Background())

	clientConfig, err := factory.ClientConfig()
	if err != nil {
		return nil, err
	}

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	velerov1api.AddToScheme(scheme)
	v1.AddToScheme(scheme)
	storagev1api.AddToScheme(scheme)

	nodeName := os.Getenv("NODE_NAME")

	// use a field selector to filter to only pods scheduled on this node.
	cacheOption := cache.Options{
		SelectorsByObject: cache.SelectorsByObject{
			&v1.Pod{}: {
				Field: fields.Set{"spec.nodeName": nodeName}.AsSelector(),
			},
		},
	}
	mgr, err := ctrl.NewManager(clientConfig, ctrl.Options{
		Scheme:   scheme,
		NewCache: cache.BuilderWithOptions(cacheOption),
	})
	if err != nil {
		return nil, err
	}

	s := &nodeAgentServer{
		logger:         logger,
		ctx:            ctx,
		cancelFunc:     cancelFunc,
		fileSystem:     filesystem.NewFileSystem(),
		mgr:            mgr,
		metricsAddress: metricAddress,
		namespace:      factory.Namespace(),
		nodeName:       nodeName,
	}

	// the cache isn't initialized yet when "validatePodVolumesHostPath" is called, the client returned by the manager cannot
	// be used, so we need the kube client here
	client, err := factory.KubeClient()
	if err != nil {
		return nil, err
	}
	if err := s.validatePodVolumesHostPath(client); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *nodeAgentServer) run() {
	signals.CancelOnShutdown(s.cancelFunc, s.logger)

	go func() {
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", promhttp.Handler())
		s.logger.Infof("Starting metric server for node agent at address [%s]", s.metricsAddress)
		if err := http.ListenAndServe(s.metricsAddress, metricsMux); err != nil {
			s.logger.Fatalf("Failed to start metric server for node agent at [%s]: %v", s.metricsAddress, err)
		}
	}()
	s.metrics = metrics.NewPodVolumeMetrics()
	s.metrics.RegisterAllMetrics()
	s.metrics.InitPodVolumeMetricsForNode(s.nodeName)

	s.markInProgressCRsFailed()

	s.logger.Info("Starting controllers")

	credentialFileStore, err := credentials.NewNamespacedFileStore(
		s.mgr.GetClient(),
		s.namespace,
		defaultCredentialsDirectory,
		filesystem.NewFileSystem(),
	)
	if err != nil {
		s.logger.Fatalf("Failed to create credentials file store: %v", err)
	}

	credSecretStore, err := credentials.NewNamespacedSecretStore(s.mgr.GetClient(), s.namespace)
	if err != nil {
		s.logger.Fatalf("Failed to create secret file store: %v", err)
	}

	credentialGetter := &credentials.CredentialGetter{FromFile: credentialFileStore, FromSecret: credSecretStore}
	pvbReconciler := controller.PodVolumeBackupReconciler{
		Scheme:           s.mgr.GetScheme(),
		Client:           s.mgr.GetClient(),
		Clock:            clocks.RealClock{},
		Metrics:          s.metrics,
		CredentialGetter: credentialGetter,
		NodeName:         s.nodeName,
		FileSystem:       filesystem.NewFileSystem(),
		Log:              s.logger,
	}
	if err := pvbReconciler.SetupWithManager(s.mgr); err != nil {
		s.logger.Fatal(err, "unable to create controller", "controller", controller.PodVolumeBackup)
	}

	if err = controller.NewPodVolumeRestoreReconciler(s.logger, s.mgr.GetClient(), credentialGetter).SetupWithManager(s.mgr); err != nil {
		s.logger.WithError(err).Fatal("Unable to create the pod volume restore controller")
	}

	s.logger.Info("Controllers starting...")

	if err := s.mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		s.logger.Fatal("Problem starting manager", err)
	}
}

// validatePodVolumesHostPath validates that the pod volumes path contains a
// directory for each Pod running on this node
func (s *nodeAgentServer) validatePodVolumesHostPath(client kubernetes.Interface) error {
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

	pods, err := client.CoreV1().Pods("").List(s.ctx, metav1.ListOptions{FieldSelector: fmt.Sprintf("spec.nodeName=%s,status.phase=Running", s.nodeName)})
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

// if there is a restarting during the reconciling of pvbs/pvrs/etc, these CRs may be stuck in progress status
// markInProgressCRsFailed tries to mark the in progress CRs as failed when starting the server to avoid the issue
func (s *nodeAgentServer) markInProgressCRsFailed() {
	// the function is called before starting the controller manager, the embedded client isn't ready to use, so create a new one here
	client, err := ctrlclient.New(s.mgr.GetConfig(), ctrlclient.Options{Scheme: s.mgr.GetScheme()})
	if err != nil {
		s.logger.WithError(errors.WithStack(err)).Error("failed to create client")
		return
	}

	s.markInProgressPVBsFailed(client)

	s.markInProgressPVRsFailed(client)
}

func (s *nodeAgentServer) markInProgressPVBsFailed(client ctrlclient.Client) {
	pvbs := &velerov1api.PodVolumeBackupList{}
	if err := client.List(s.ctx, pvbs, &ctrlclient.MatchingFields{"metadata.namespace": s.namespace}); err != nil {
		s.logger.WithError(errors.WithStack(err)).Error("failed to list podvolumebackups")
		return
	}
	for i, pvb := range pvbs.Items {
		if pvb.Status.Phase != velerov1api.PodVolumeBackupPhaseInProgress {
			s.logger.Debugf("the status of podvolumebackup %q is %q, skip", pvb.GetName(), pvb.Status.Phase)
			continue
		}
		if pvb.Spec.Node != s.nodeName {
			s.logger.Debugf("the node of podvolumebackup %q is %q, not %q, skip", pvb.GetName(), pvb.Spec.Node, s.nodeName)
			continue
		}

		if err := controller.UpdatePVBStatusToFailed(client, s.ctx, &pvbs.Items[i],
			fmt.Sprintf("get a podvolumebackup with status %q during the server starting, mark it as %q", velerov1api.PodVolumeBackupPhaseInProgress, velerov1api.PodVolumeBackupPhaseFailed),
			time.Now()); err != nil {
			s.logger.WithError(errors.WithStack(err)).Errorf("failed to patch podvolumebackup %q", pvb.GetName())
			continue
		}
		s.logger.WithField("podvolumebackup", pvb.GetName()).Warn(pvb.Status.Message)
	}
}

func (s *nodeAgentServer) markInProgressPVRsFailed(client ctrlclient.Client) {
	pvrs := &velerov1api.PodVolumeRestoreList{}
	if err := client.List(s.ctx, pvrs, &ctrlclient.MatchingFields{"metadata.namespace": s.namespace}); err != nil {
		s.logger.WithError(errors.WithStack(err)).Error("failed to list podvolumerestores")
		return
	}
	for i, pvr := range pvrs.Items {
		if pvr.Status.Phase != velerov1api.PodVolumeRestorePhaseInProgress {
			s.logger.Debugf("the status of podvolumerestore %q is %q, skip", pvr.GetName(), pvr.Status.Phase)
			continue
		}

		pod := &v1.Pod{}
		if err := client.Get(s.ctx, types.NamespacedName{
			Namespace: pvr.Spec.Pod.Namespace,
			Name:      pvr.Spec.Pod.Name,
		}, pod); err != nil {
			s.logger.WithError(errors.WithStack(err)).Errorf("failed to get pod \"%s/%s\" of podvolumerestore %q",
				pvr.Spec.Pod.Namespace, pvr.Spec.Pod.Name, pvr.GetName())
			continue
		}
		if pod.Spec.NodeName != s.nodeName {
			s.logger.Debugf("the node of pod referenced by podvolumebackup %q is %q, not %q, skip", pvr.GetName(), pod.Spec.NodeName, s.nodeName)
			continue
		}

		if err := controller.UpdatePVRStatusToFailed(client, s.ctx, &pvrs.Items[i],
			fmt.Sprintf("get a podvolumerestore with status %q during the server starting, mark it as %q", velerov1api.PodVolumeRestorePhaseInProgress, velerov1api.PodVolumeRestorePhaseFailed),
			time.Now()); err != nil {
			s.logger.WithError(errors.WithStack(err)).Errorf("failed to patch podvolumerestore %q", pvr.GetName())
			continue
		}
		s.logger.WithField("podvolumerestore", pvr.GetName()).Warn(pvr.Status.Message)
	}
}
