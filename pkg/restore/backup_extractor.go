/*
Copyright 2019 the Velero contributors.

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

package restore

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/heptio/velero/pkg/util/filesystem"
)

// backupExtractor unzips/extracts a backup tarball to a local
// temp directory.
type backupExtractor struct {
	log        logrus.FieldLogger
	fileSystem filesystem.Interface
}

// unzipAndExtractBackup extracts a reader on a gzipped tarball to a local temp directory
func (e *backupExtractor) unzipAndExtractBackup(src io.Reader) (string, error) {
	gzr, err := gzip.NewReader(src)
	if err != nil {
		e.log.Infof("error creating gzip reader: %v", err)
		return "", err
	}
	defer gzr.Close()

	return e.readBackup(tar.NewReader(gzr))
}

func (e *backupExtractor) writeFile(target string, tarRdr *tar.Reader) error {
	file, err := e.fileSystem.Create(target)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := io.Copy(file, tarRdr); err != nil {
		return err
	}
	return nil
}

func (e *backupExtractor) readBackup(tarRdr *tar.Reader) (string, error) {
	dir, err := e.fileSystem.TempDir("", "")
	if err != nil {
		e.log.Infof("error creating temp dir: %v", err)
		return "", err
	}

	for {
		header, err := tarRdr.Next()

		if err == io.EOF {
			break
		}
		if err != nil {
			e.log.Infof("error reading tar: %v", err)
			return "", err
		}

		target := filepath.Join(dir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			err := e.fileSystem.MkdirAll(target, header.FileInfo().Mode())
			if err != nil {
				e.log.Infof("mkdirall error: %v", err)
				return "", err
			}

		case tar.TypeReg:
			// make sure we have the directory created
			err := e.fileSystem.MkdirAll(filepath.Dir(target), header.FileInfo().Mode())
			if err != nil {
				e.log.Infof("mkdirall error: %v", err)
				return "", err
			}

			// create the file
			if err := e.writeFile(target, tarRdr); err != nil {
				e.log.Infof("error copying: %v", err)
				return "", err
			}
		}
	}

	return dir, nil
}
