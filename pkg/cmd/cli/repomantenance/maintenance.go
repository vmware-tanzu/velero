package repomantenance

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/internal/credentials"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerocli "github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/repository"
	"github.com/vmware-tanzu/velero/pkg/repository/provider"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
	"github.com/vmware-tanzu/velero/pkg/util/logging"
)

type Options struct {
	RepoName              string
	BackupStorageLocation string
	RepoType              string
	LogLevelFlag          *logging.LevelFlag
	FormatFlag            *logging.FormatFlag
}

func (o *Options) BindFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.RepoName, "repo-name", "", "namespace of the pod/volume that the snapshot is for")
	flags.StringVar(&o.BackupStorageLocation, "backup-storage-location", "", "backup's storage location name")
	flags.StringVar(&o.RepoType, "repo-type", velerov1api.BackupRepositoryTypeKopia, "type of the repository where the snapshot is stored")
	flags.Var(o.LogLevelFlag, "log-level", fmt.Sprintf("The level at which to log. Valid values are %s.", strings.Join(o.LogLevelFlag.AllowedValues(), ", ")))
	flags.Var(o.FormatFlag, "log-format", fmt.Sprintf("The format for log output. Valid values are %s.", strings.Join(o.FormatFlag.AllowedValues(), ", ")))
}

func NewCommand(f velerocli.Factory) *cobra.Command {
	o := &Options{
		LogLevelFlag: logging.LogLevelFlag(logrus.InfoLevel),
		FormatFlag:   logging.NewFormatFlag(),
	}
	cmd := &cobra.Command{
		Use:    "repo-maintenance",
		Hidden: true,
		Short:  "VELERO INTERNAL COMMAND ONLY - not intended to be run directly by users",
		Run: func(c *cobra.Command, args []string) {
			o.Run(f)
		},
	}

	o.BindFlags(cmd.Flags())
	return cmd
}

func (o *Options) Run(f velerocli.Factory) {
	logger := logging.DefaultLogger(o.LogLevelFlag.Parse(), o.FormatFlag.Parse())
	logger.SetOutput(os.Stdout)

	pruneError := o.runRepoPrune(f, f.Namespace(), logger)
	defer func() {
		if pruneError != nil {
			os.Exit(1)
		}
	}()

	if pruneError != nil {
		logger.WithError(pruneError).Error("An error occurred when running repo prune")
		terminationLogFile, err := os.Create("/dev/termination-log")
		if err != nil {
			logger.WithError(err).Error("Failed to create termination log file")
			return
		}
		defer terminationLogFile.Close()

		if _, errWrite := terminationLogFile.WriteString(fmt.Sprintf("An error occurred: %v", err)); errWrite != nil {
			logger.WithError(errWrite).Error("Failed to write error to termination log file")
		}
	}
}

func (o *Options) initClient(f velerocli.Factory) (client.Client, error) {
	scheme := runtime.NewScheme()
	err := velerov1api.AddToScheme(scheme)
	if err != nil {
		return nil, errors.Wrap(err, "failed to add velero scheme")
	}

	err = v1.AddToScheme(scheme)
	if err != nil {
		return nil, errors.Wrap(err, "failed to add api core scheme")
	}

	config, err := f.ClientConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get client config")
	}

	cli, err := client.New(config, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create client")
	}

	return cli, nil
}

func (o *Options) runRepoPrune(f velerocli.Factory, namespace string, logger logrus.FieldLogger) error {
	cli, err := o.initClient(f)
	if err != nil {
		return err
	}

	credentialFileStore, err := credentials.NewNamespacedFileStore(
		cli,
		namespace,
		"/tmp/credentials",
		filesystem.NewFileSystem(),
	)
	if err != nil {
		return errors.Wrap(err, "failed to create namespaced file store")
	}

	credentialSecretStore, err := credentials.NewNamespacedSecretStore(cli, namespace)
	if err != nil {
		return errors.Wrap(err, "failed to create namespaced secret store")
	}

	var repoProvider provider.Provider
	if o.RepoType == velerov1api.BackupRepositoryTypeRestic {
		repoProvider = provider.NewResticRepositoryProvider(credentialFileStore, filesystem.NewFileSystem(), logger)
	} else {
		repoProvider = provider.NewUnifiedRepoProvider(
			credentials.CredentialGetter{
				FromFile:   credentialFileStore,
				FromSecret: credentialSecretStore,
			}, o.RepoType, logger)
	}

	// backupRepository
	repo, err := repository.GetBackupRepository(context.Background(), cli, namespace,
		repository.BackupRepositoryKey{
			VolumeNamespace: o.RepoName,
			BackupLocation:  o.BackupStorageLocation,
			RepositoryType:  o.RepoType,
		}, true)

	if err != nil {
		return errors.Wrap(err, "failed to get backup repository")
	}

	// bsl
	bsl := &velerov1api.BackupStorageLocation{}
	err = cli.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: repo.Spec.BackupStorageLocation}, bsl)
	if err != nil {
		return errors.Wrap(err, "failed to get backup storage location")
	}

	para := provider.RepoParam{
		BackupRepo:     repo,
		BackupLocation: bsl,
	}

	err = repoProvider.BoostRepoConnect(context.Background(), para)
	if err != nil {
		return errors.Wrap(err, "failed to boost repo connect")
	}

	err = repoProvider.PruneRepo(context.Background(), para)
	if err != nil {
		return errors.Wrap(err, "failed to prune repo")
	}
	return nil
}
