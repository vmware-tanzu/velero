/*
Copyright 2018 the Velero contributors.

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
	"io/ioutil"
	"os"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/heptio/velero/pkg/buildinfo"
	"github.com/heptio/velero/pkg/client"
	"github.com/heptio/velero/pkg/cmd"
	"github.com/heptio/velero/pkg/cmd/util/signals"
	"github.com/heptio/velero/pkg/controller"
	clientset "github.com/heptio/velero/pkg/generated/clientset/versioned"
	informers "github.com/heptio/velero/pkg/generated/informers/externalversions"
	"github.com/heptio/velero/pkg/restic"
	"github.com/heptio/velero/pkg/util/logging"
)

func NewServerCommand(f client.Factory) *cobra.Command {
	logLevelFlag := logging.LogLevelFlag(logrus.InfoLevel)

	command := &cobra.Command{
		Use:    "server",
		Short:  "Run the velero restic server",
		Long:   "Run the velero restic server",
		Hidden: true,
		Run: func(c *cobra.Command, args []string) {
			logLevel := logLevelFlag.Parse()
			logrus.Infof("Setting log-level to %s", strings.ToUpper(logLevel.String()))

			logger := logging.DefaultLogger(logLevel)
			logger.Infof("Starting Velero restic server %s (%s)", buildinfo.Version, buildinfo.FormattedGitSHA())

			s, err := newResticServer(logger, fmt.Sprintf("%s-%s", c.Parent().Name(), c.Name()))
			cmd.CheckError(err)

			s.run()
		},
	}

	command.Flags().Var(logLevelFlag, "log-level", fmt.Sprintf("the level at which to log. Valid values are %s.", strings.Join(logLevelFlag.AllowedValues(), ", ")))

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

	err = validatePodVolumesHostPath(kubeClient, "/host_pods/")
	if err != nil {
		return nil, errors.WithStack(err)
	}

	veleroClient, err := clientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.WithStack(err)
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
		os.Getenv("VELERO_NAMESPACE"),
		0,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		func(opts *metav1.ListOptions) {
			opts.FieldSelector = fmt.Sprintf("metadata.name=%s", restic.CredentialsSecretName)
		},
	)

	ctx, cancelFunc := context.WithCancel(context.Background())

	return &resticServer{
		kubeClient:            kubeClient,
		veleroClient:          veleroClient,
		veleroInformerFactory: informers.NewFilteredSharedInformerFactory(veleroClient, 0, os.Getenv("VELERO_NAMESPACE"), nil),
		kubeInformerFactory:   kubeinformers.NewSharedInformerFactory(kubeClient, 0),
		podInformer:           podInformer,
		secretInformer:        secretInformer,
		logger:                logger,
		ctx:                   ctx,
		cancelFunc:            cancelFunc,
	}, nil
}

func (s *resticServer) run() {
	signals.CancelOnShutdown(s.cancelFunc, s.logger)

	s.logger.Info("Starting controllers")

	var wg sync.WaitGroup

	backupController := controller.NewPodVolumeBackupController(
		s.logger,
		s.veleroInformerFactory.Velero().V1().PodVolumeBackups(),
		s.veleroClient.VeleroV1(),
		s.podInformer,
		s.secretInformer,
		s.kubeInformerFactory.Core().V1().PersistentVolumeClaims(),
		s.veleroInformerFactory.Velero().V1().BackupStorageLocations(),
		os.Getenv("NODE_NAME"),
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		backupController.Run(s.ctx, 1)
	}()

	restoreController := controller.NewPodVolumeRestoreController(
		s.logger,
		s.veleroInformerFactory.Velero().V1().PodVolumeRestores(),
		s.veleroClient.VeleroV1(),
		s.podInformer,
		s.secretInformer,
		s.kubeInformerFactory.Core().V1().PersistentVolumeClaims(),
		s.veleroInformerFactory.Velero().V1().BackupStorageLocations(),
		os.Getenv("NODE_NAME"),
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		restoreController.Run(s.ctx, 1)
	}()

	go s.veleroInformerFactory.Start(s.ctx.Done())
	go s.kubeInformerFactory.Start(s.ctx.Done())
	go s.podInformer.Run(s.ctx.Done())
	go s.secretInformer.Run(s.ctx.Done())

	s.logger.Info("Controllers started successfully")

	<-s.ctx.Done()

	s.logger.Info("Waiting for all controllers to shut down gracefully")
	wg.Wait()
}

// validatePodVolumesHostPath validates that the podVolumesPath contains a
// directory for each Pod running on this node
func validatePodVolumesHostPath(kubeClient kubernetes.Interface, podVolumesPath string) error {
	files, err := ioutil.ReadDir(podVolumesPath)
	if err != nil {
		return errors.WithStack(err)
	}

	// create a map of directory names inside the podVolumesPath
	dirs := map[string]bool{}
	for _, f := range files {
		if f.IsDir() {
			dirs[f.Name()] = true
		}
	}

	pods, err := kubeClient.CoreV1().Pods("").List(metav1.ListOptions{FieldSelector: fmt.Sprintf("spec.nodeName=%s", os.Getenv("NODE_NAME"))})
	if err != nil {
		return errors.WithStack(err)
	}

	for _, pod := range pods.Items {
		dirName := string(pod.GetUID())

		// if the pod is a mirror pod, the directory name is the hash value of the
		// mirror pod annotation
		if hash, ok := pod.GetAnnotations()[v1.MirrorPodAnnotationKey]; ok {
			dirName = hash
		}

		if _, ok := dirs[dirName]; !ok {
			return errors.WithStack(fmt.Errorf("could not find volumes for pod %s/%s in hostpath, check if the pod volumes hostPath mount is correct", pod.GetNamespace(), pod.GetName()))
		}
	}
	return nil
}
