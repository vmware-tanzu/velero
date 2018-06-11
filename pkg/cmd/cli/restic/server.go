package restic

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/heptio/ark/pkg/buildinfo"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cmd"
	"github.com/heptio/ark/pkg/cmd/util/signals"
	"github.com/heptio/ark/pkg/controller"
	clientset "github.com/heptio/ark/pkg/generated/clientset/versioned"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions"
	"github.com/heptio/ark/pkg/util/logging"
)

func NewServerCommand(f client.Factory) *cobra.Command {
	var logLevelFlag = logging.LogLevelFlag(logrus.InfoLevel)

	var command = &cobra.Command{
		Use:   "server",
		Short: "Run the ark restic server",
		Long:  "Run the ark restic server",
		Run: func(c *cobra.Command, args []string) {
			logLevel := logLevelFlag.Parse()
			logrus.Infof("Setting log-level to %s", strings.ToUpper(logLevel.String()))

			logger := logging.DefaultLogger(logLevel)
			logger.Infof("Starting Ark restic server %s", buildinfo.FormattedGitSHA())

			s, err := newResticServer(logger, fmt.Sprintf("%s-%s", c.Parent().Name(), c.Name()))
			cmd.CheckError(err)

			s.run()
		},
	}

	command.Flags().Var(logLevelFlag, "log-level", fmt.Sprintf("the level at which to log. Valid values are %s.", strings.Join(logLevelFlag.AllowedValues(), ", ")))

	return command
}

type resticServer struct {
	kubeClient          kubernetes.Interface
	arkClient           clientset.Interface
	arkInformerFactory  informers.SharedInformerFactory
	kubeInformerFactory kubeinformers.SharedInformerFactory
	podInformer         cache.SharedIndexInformer
	logger              logrus.FieldLogger
	ctx                 context.Context
	cancelFunc          context.CancelFunc
}

func newResticServer(logger logrus.FieldLogger, baseName string) (*resticServer, error) {
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

	// use a stand-alone pod informer because we want to use a field selector to
	// filter to only pods scheduled on this node.
	podInformer := corev1informers.NewFilteredPodInformer(
		kubeClient,
		"",
		0,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		func(opts *metav1.ListOptions) {
			opts.FieldSelector = fmt.Sprintf("spec.nodeName=%s", os.Getenv("NODE_NAME"))
		},
	)

	ctx, cancelFunc := context.WithCancel(context.Background())

	return &resticServer{
		kubeClient:          kubeClient,
		arkClient:           arkClient,
		arkInformerFactory:  informers.NewFilteredSharedInformerFactory(arkClient, 0, os.Getenv("HEPTIO_ARK_NAMESPACE"), nil),
		kubeInformerFactory: kubeinformers.NewSharedInformerFactory(kubeClient, 0),
		podInformer:         podInformer,
		logger:              logger,
		ctx:                 ctx,
		cancelFunc:          cancelFunc,
	}, nil
}

func (s *resticServer) run() {
	signals.CancelOnShutdown(s.cancelFunc, s.logger)

	s.logger.Info("Starting controllers")

	var wg sync.WaitGroup

	backupController := controller.NewPodVolumeBackupController(
		s.logger,
		s.arkInformerFactory.Ark().V1().PodVolumeBackups(),
		s.arkClient.ArkV1(),
		s.podInformer,
		s.kubeInformerFactory.Core().V1().Secrets(),
		s.kubeInformerFactory.Core().V1().PersistentVolumeClaims(),
		os.Getenv("NODE_NAME"),
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		backupController.Run(s.ctx, 1)
	}()

	restoreController := controller.NewPodVolumeRestoreController(
		s.logger,
		s.arkInformerFactory.Ark().V1().PodVolumeRestores(),
		s.arkClient.ArkV1(),
		s.podInformer,
		s.kubeInformerFactory.Core().V1().Secrets(),
		s.kubeInformerFactory.Core().V1().PersistentVolumeClaims(),
		os.Getenv("NODE_NAME"),
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		restoreController.Run(s.ctx, 1)
	}()

	go s.arkInformerFactory.Start(s.ctx.Done())
	go s.kubeInformerFactory.Start(s.ctx.Done())
	go s.podInformer.Run(s.ctx.Done())

	s.logger.Info("Controllers started successfully")

	<-s.ctx.Done()

	s.logger.Info("Waiting for all controllers to shut down gracefully")
	wg.Wait()
}
