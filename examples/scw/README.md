# Scaleway bucket

To use Scaleway.com object storage, that is S3 comptible, you only need to:

- create a bucket named "velero" in your object storage dashboard
- create an access key in your profile panel to get access key and secret key
- set the given keys in `config/scw/00-scw-credentials.yaml`

Then you `kubect apply -f config/scw`.

At this time, only the `nl-ams` region is usable. You will be able to change that later in `config/scw/05-backupstoragelocation.yaml` file.
