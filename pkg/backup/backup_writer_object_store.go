package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

type readWriteSeekCloser interface {
	io.Reader
	io.Writer
	io.Closer
	io.Seeker
}

type objectStoreBackupWriter struct {
	objectStore     cloudprovider.ObjectStore
	bucket          string
	prefix          string
	objectMarshaler objectMarshaler

	backupFile     readWriteSeekCloser
	backupFileName string
	gzipWriter     io.WriteCloser
	tarWriter      TarWriter
}

func NewObjectStoreBackupWriter(objectStore cloudprovider.ObjectStore, bucket, prefix string, objectMarshaler objectMarshaler) Writer {
	return &objectStoreBackupWriter{
		objectStore:     objectStore,
		bucket:          bucket,
		prefix:          prefix,
		objectMarshaler: objectMarshaler,
	}
}

func (w *objectStoreBackupWriter) PrepareBackup(backup *v1.Backup) error {
	backupFile, err := ioutil.TempFile("", "backup")
	if err != nil {
		return err
	}
	w.backupFileName = backupFile.Name()
	w.backupFile = backupFile
	w.gzipWriter = gzip.NewWriter(w.backupFile)
	w.tarWriter = tar.NewWriter(w.gzipWriter)
	return nil
}

func resourceFilePath(id ResourceIdentifier, extension string) string {
	return filepath.Join(
		v1.ResourcesDir,
		id.GroupResource.String(),
		scopeDir(id.Namespace),
		id.Namespace,
		fmt.Sprintf("%s.%s", id.Name, extension),
	)
}

func scopeDir(namespace string) string {
	if namespace == "" {
		return v1.NamespaceScopedDir
	}
	return v1.ClusterScopedDir
}

func (w *objectStoreBackupWriter) WriteResource(id ResourceIdentifier, obj runtime.Unstructured) error {
	filePath := resourceFilePath(id, w.objectMarshaler.Extension())

	data, err := w.objectMarshaler.Marshal(obj.UnstructuredContent())
	if err != nil {
		return err
	}

	hdr := &tar.Header{
		Name:     filePath,
		Size:     int64(len(data)),
		Typeflag: tar.TypeReg,
		Mode:     0755,
		ModTime:  time.Now(),
	}

	if err := w.tarWriter.WriteHeader(hdr); err != nil {
		return errors.WithStack(err)
	}

	if _, err := w.tarWriter.Write(data); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (w *objectStoreBackupWriter) closeAndRemoveBackupFile(err error) error {
	closeErr := w.backupFile.Close()

	removeErr := os.Remove(w.backupFileName)

	if err != nil {
		return err
	}
}

func (w *objectStoreBackupWriter) FinalizeBackup(backup *v1.Backup) error {
	tarErr := w.tarWriter.Close()
	gzipErr := w.gzipWriter.Close()

	if tarErr != nil {
		// Try to remove the temp file
		if removeErr := os.Remove(w.backupFileName); removeErr != nil {
			// log
		}
		return errors.WithStack(tarErr)
	}

	if gzipErr != nil {
		// Try to remove the temp file
		if removeErr := os.Remove(w.backupFileName); removeErr != nil {
			// log
		}
		return errors.WithStack(gzipErr)
	}

	if _, err := w.backupFile.Seek(0, 0); err != nil {
		// Try to remove the temp file
		if removeErr := os.Remove(w.backupFileName); removeErr != nil {
			// log
		}
		return errors.WithStack(err)
	}

	key := fmt.Sprintf("%s%s/%s.tar.gz", w.prefix, backup.Name, backup.Name)
	putErr := w.objectStore.PutObject(w.bucket, key, w.backupFile)

	// Try to remove the temp file
	removeErr := os.Remove(w.backupFileName)
	if removeErr != nil {
		// log
	}

	if putErr != nil {
		return putErr
	}

	return removeErr
}

type TarWriter interface {
	io.Closer
	Write([]byte) (int, error)
	WriteHeader(*tar.Header) error
}
