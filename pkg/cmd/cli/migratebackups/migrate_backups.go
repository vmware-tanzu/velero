package migratebackups

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"path"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/client"
	"github.com/heptio/velero/pkg/cloudprovider"
	"github.com/heptio/velero/pkg/persistence"
	"github.com/heptio/velero/pkg/plugin"
	"github.com/heptio/velero/pkg/util/encode"
	"github.com/heptio/velero/pkg/volume"
)

func NewCommand(f client.Factory) *cobra.Command {
	var (
		backupLocation   = "default"
		snapshotLocation = ""
	)

	c := &cobra.Command{
		Use:   "migrate-backups",
		Short: "Rewrite metadata for backups created before v0.11 to match the current format in object storage",
		Long: `
The "velero migrate-backups" command rewrites metadata for backups created before v0.11 to match the current format in object storage. It
does this by converting the ark-backup.json files to velero-backup.json files, and creating <backup name>-volumesnapshots.json.gz 
files to replace the contents of backups' status.volumeBackups field.

"velero migrate-backups" is *ONLY* appropriate for use if you're running velero v0.11.x on the server, and you've already followed the migration steps here:
    https://heptio.github.io/velero/v0.11.0/migrating-to-velero.`,
		Example: `  # rewrite any pre-v0.11 backups in the backup storage location named "default"
  velero migrate-backups --backup-location default

  # rewrite pre-v0.10 backups in the backup storage location named "default", and indicate that
  # the volume snapshots associated with the backup are in the volume snapshot location named "foo"
  velero migrate-backups --backup-location default --snapshot-location foo
		`,
		Run: func(c *cobra.Command, args []string) {
			log.Println("+ PREPARING")

			log.Println("   + getting Velero API client")
			client, err := f.Client()
			if err != nil {
				log.Fatalf("   + error getting Velero API client! %v\n", err)
			}

			log.Printf("   + getting backup storage location %q\n", backupLocation)
			bsl, err := client.VeleroV1().BackupStorageLocations(f.Namespace()).Get(backupLocation, metav1.GetOptions{})
			if err != nil {
				log.Fatalf("   + error getting backup storage location! %v\n", err)
			}

			log.Println("   + discovering plugins")
			logger := logrus.StandardLogger()
			logger.Out = ioutil.Discard
			registry := plugin.NewRegistry("", logger, logrus.InfoLevel)
			if err := registry.DiscoverPlugins(); err != nil {
				log.Fatalf("   + error discovering plugins! %v\n", err)
			}

			log.Println("   + creating plugin manager")
			pluginManager := plugin.NewManager(logger, logrus.InfoLevel, registry)
			defer pluginManager.CleanupClients()

			log.Println("   + getting object store plugin")
			objectStore, err := pluginManager.GetObjectStore(bsl.Spec.Provider)
			if err != nil {
				log.Fatalf("   +error getting object store plugin! %v\n", err)
			}

			log.Println("   + initializing object store plugin")
			if err := objectStore.Init(bsl.Spec.Config); err != nil {
				log.Fatalf("   +error initializing object store plugin! %v\n", err)
			}

			log.Println("   + getting backup store")
			backupStore, err := persistence.NewObjectBackupStore(bsl, pluginManager, logger)
			if err != nil {
				log.Fatalf("   +error getting backup store! %v\n", err)
			}

			log.Println("   + listing backups in backup store")
			backupNames, err := backupStore.ListBackups()
			if err != nil {
				log.Fatalf("   + error listing backups! %v\n", err)
			}

			log.Println("+ PROCESSING BACKUPS")
			var totalCount, currentCount, succeededCount int
			for _, backupName := range backupNames {
				totalCount++

				log.Printf("   + processing backup %q\n", backupName)

				log.Println("      + checking whether backup is in the current format")
				current, err := isCurrent(backupName, objectStore, bsl)
				if err != nil {
					log.Printf("      + error checking whether backup is in the current format! %v\n", err)
					continue
				}
				if current {
					currentCount++
					log.Printf("      + backup is already in the current format!\n")
					continue
				}

				log.Printf("      + starting migration of backup to current format\n")
				if err := migrateBackup(backupName, backupStore, objectStore, bsl, snapshotLocation); err != nil {
					log.Printf("      + error migrating backup! %v\n", err)
					continue
				}
				succeededCount++
				log.Printf("      + migration of backup complete!\n")
			}
			log.Printf("+ MIGRATION SUMMARY\n")
			log.Printf("   + total backups processed:\t%d\n", totalCount)
			log.Printf("   + already current backups:\t%d\n", currentCount)
			log.Printf("   + successful migrations:\t\t%d\n", succeededCount)
			log.Printf("   + failed migrations:\t\t%d\n", totalCount-currentCount-succeededCount)
		},
	}

	c.Flags().StringVar(&backupLocation, "backup-location", backupLocation, "backup storage location to process migration for")
	c.Flags().StringVar(&snapshotLocation, "snapshot-location", snapshotLocation, "volume snapshot location to assign volume snapshots to (if found in backup.status.volumeBackups)")

	return c
}

func isCurrent(backupName string, objectStore cloudprovider.ObjectStore, bsl *v1.BackupStorageLocation) (bool, error) {
	prefix := path.Join(bsl.Spec.ObjectStorage.Prefix, "backups", backupName) + "/"
	keys, err := objectStore.ListObjects(bsl.Spec.ObjectStorage.Bucket, prefix)
	if err != nil {
		return false, errors.Wrapf(err, "error listing objects in bucket %q under prefix %q", bsl.Spec.ObjectStorage.Bucket, prefix)
	}

	currentMetadataKey := prefix + "velero-backup.json"
	for _, key := range keys {
		if key == currentMetadataKey {
			return true, nil
		}
	}

	return false, nil
}

func migrateBackup(
	backupName string,
	backupStore persistence.BackupStore,
	objectStore cloudprovider.ObjectStore,
	bsl *v1.BackupStorageLocation,
	vslName string,
) error {
	log.Printf("      + getting ark-backup.json metadata file from object storage\n")
	backup, err := backupStore.GetBackupMetadata(backupName)
	if err != nil {
		return errors.Wrap(err, "error getting backup metadata file from object storage")
	}

	var volumeSnapshots []*volume.Snapshot
	log.Printf("      + processing backup.status.volumeBackups\n")
	for pvName, snapshotInfo := range backup.Status.VolumeBackups {
		log.Printf("         + processing backup.status.volumeBackups[%s]\n", pvName)
		volumeSnapshot := &volume.Snapshot{
			Spec: volume.SnapshotSpec{
				BackupName:           backupName,
				BackupUID:            string(backup.UID),
				Location:             vslName,
				PersistentVolumeName: pvName,
				VolumeType:           snapshotInfo.Type,
				VolumeAZ:             snapshotInfo.AvailabilityZone,
				VolumeIOPS:           snapshotInfo.Iops,
			},
			Status: volume.SnapshotStatus{
				ProviderSnapshotID: snapshotInfo.SnapshotID,
				Phase:              volume.SnapshotPhaseCompleted,
			},
		}

		volumeSnapshots = append(volumeSnapshots, volumeSnapshot)
	}

	backup.Status.VolumeSnapshotsAttempted = len(volumeSnapshots)
	backup.Status.VolumeSnapshotsCompleted = len(volumeSnapshots)

	backup.Status.VolumeBackups = nil

	log.Printf("      + encoding volume snapshots as %s-volumesnapshots.json.gz\n", backupName)
	volumeSnapshotsReader, err := encodeVolumeSnapshots(volumeSnapshots)
	if err != nil {
		return errors.Wrap(err, "error writing volume snapshots to gzipped JSON file")
	}

	log.Printf("      + uploading %s-volumesnapshots.json.gz to object storage\n", backupName)
	volumeSnapshotsKey := path.Join(bsl.Spec.ObjectStorage.Prefix, "backups", backupName, backupName+"-volumesnapshots.json.gz")
	if err := objectStore.PutObject(bsl.Spec.ObjectStorage.Bucket, volumeSnapshotsKey, volumeSnapshotsReader); err != nil {
		return errors.Wrap(err, "error uploading volumesnapshots.json.gz file to object storage")
	}

	log.Printf("      + encoding backup metadata to velero-backup.json\n")
	backupMetadataReader, err := encodeBackup(backup)
	if err != nil {
		return errors.Wrap(err, "error encoding backup metadata to JSON")
	}

	log.Printf("      + uploading velero-backup.json to object storage\n")
	backupMetadataKey := path.Join(bsl.Spec.ObjectStorage.Prefix, "backups", backup.Name, "velero-backup.json")
	if err := objectStore.PutObject(bsl.Spec.ObjectStorage.Bucket, backupMetadataKey, backupMetadataReader); err != nil {
		return errors.Wrap(err, "error uploading velero-backup.json file to object storage")
	}

	log.Printf("      + deleting ark-backup.json from object storage\n")
	legacyMetadataKey := path.Join(bsl.Spec.ObjectStorage.Prefix, "backups", backup.Name, "ark-backup.json")
	if err := objectStore.DeleteObject(bsl.Spec.ObjectStorage.Bucket, legacyMetadataKey); err != nil {
		return errors.Wrap(err, "error deleting ark-backup.json file from object storage")
	}

	return nil
}

func encodeVolumeSnapshots(volumeSnapshots []*volume.Snapshot) (io.Reader, error) {
	buf := new(bytes.Buffer)
	gzw := gzip.NewWriter(buf)
	defer gzw.Close()

	if err := json.NewEncoder(gzw).Encode(volumeSnapshots); err != nil {
		return nil, err
	}

	if err := gzw.Close(); err != nil {
		return nil, err
	}

	return buf, nil
}

func encodeBackup(backup *v1.Backup) (io.Reader, error) {
	buf := new(bytes.Buffer)

	if err := encode.EncodeTo(backup, "json", buf); err != nil {
		return nil, err
	}

	return buf, nil
}
