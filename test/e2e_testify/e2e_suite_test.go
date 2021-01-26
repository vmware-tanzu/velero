package e2etestify

import (
	"flag"
)

var (
	bslBucket            = flag.String("bucket", "", "name of the object storage bucket where backups from e2e tests should be stored. Required.")
	bslConfig            = flag.String("bsl-config", "", "configuration to use for the backup storage location. Format is key1=value1,key2=value2")
	bslPrefix            = flag.String("prefix", "", "prefix under which all Velero data should be stored within the bucket. Optional.")
	cloudCredentialsFile = flag.String("credentials-file", "", "file containing credentials for backup and volume provider. Required.")
	pluginProvider       = flag.String("plugin-provider", "", "Provider of object store and volume snapshotter plugins. Required.")
	veleroCLI            = flag.String("velerocli", "velero", "path to the velero application to use.")
	veleroImage          = flag.String("velero-image", "velero/velero:main", "image for the velero server to be tested.")
	vslConfig            = flag.String("vsl-config", "", "configuration to use for the volume snapshot location. Format is key1=value1,key2=value2")
)
