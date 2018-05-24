package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/pkg/errors"
)

type readWriteSeekCloser interface {
	io.Reader
	io.Writer
	io.Closer
	io.Seeker
}

type objectStoreBackupWriter struct {
	objectStore cloudprovider.ObjectStore
	bucket      string
	prefix      string

	backupFile readWriteSeekCloser
	gzipWriter io.WriteCloser
	tarWriter  TarWriter
}

func NewObjectStoreBackupWriter(objectStore cloudprovider.ObjectStore, bucket, prefix string) Writer {
	return &objectStoreBackupWriter{
		objectStore: objectStore,
		bucket:      bucket,
		prefix:      prefix,
	}
}

func (w *objectStoreBackupWriter) PrepareBackup(backup *v1.Backup) error {
	backupFile, err := ioutil.TempFile("", "backup")
	if err != nil {
		return err
	}
	w.backupFile = backupFile
	w.gzipWriter = gzip.NewWriter(w.backupFile)
	w.tarWriter = tar.NewWriter(w.gzipWriter)
	return nil
}

func (w *objectStoreBackupWriter) WriteResource(groupResource, namespace, name string, resource []byte) error {
	var filePath string
	if namespace != "" {
		filePath = filepath.Join(v1.ResourcesDir, groupResource, v1.NamespaceScopedDir, namespace, name+".json")
	} else {
		filePath = filepath.Join(v1.ResourcesDir, groupResource, v1.ClusterScopedDir, name+".json")
	}

	hdr := &tar.Header{
		Name:     filePath,
		Size:     int64(len(resource)),
		Typeflag: tar.TypeReg,
		Mode:     0755,
		ModTime:  time.Now(),
	}

	if err := w.tarWriter.WriteHeader(hdr); err != nil {
		return errors.WithStack(err)
	}

	if _, err := w.tarWriter.Write(resource); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (w *objectStoreBackupWriter) FinalizeBackup(backup *v1.Backup) error {
	tarErr := w.tarWriter.Close()
	gzipErr := w.gzipWriter.Close()

	if tarErr != nil {
		return tarErr
	}
	if gzipErr != nil {
		return gzipErr
	}

	if _, err := w.backupFile.Seek(0, 0); err != nil {
		// todo remove temp file
		return err
	}

	key := fmt.Sprintf("%s%s/%s.tar.gz", w.prefix, backup.Name, backup.Name)
	if err := w.objectStore.PutObject(w.bucket, key, w.backupFile); err != nil {
		// todo remove temp file
		return err
	}

	return nil
}

type TarWriter interface {
	io.Closer
	Write([]byte) (int, error)
	WriteHeader(*tar.Header) error
}
