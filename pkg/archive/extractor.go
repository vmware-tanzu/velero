/*
Copyright the Velero contributors.

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

package archive

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
)

// Extractor unzips/extracts a backup tarball to a local
// temp directory.
type Extractor struct {
	log logrus.FieldLogger
	fs  filesystem.Interface
}

func NewExtractor(log logrus.FieldLogger, fs filesystem.Interface) *Extractor {
	return &Extractor{
		log: log,
		fs:  fs,
	}
}

// UnzipAndExtractBackup extracts a reader on a gzipped tarball to a local temp directory
func (e *Extractor) UnzipAndExtractBackup(src io.Reader) (string, map[string]string, error) {
	gzr, err := gzip.NewReader(src)
	if err != nil {
		e.log.Infof("error creating gzip reader: %v", err)
		return "", nil, err
	}
	defer gzr.Close()

	return e.readBackup(tar.NewReader(gzr))
}

func (e *Extractor) writeFile(target string, tarRdr *tar.Reader) error {
	file, err := e.fs.Create(target)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := io.Copy(file, tarRdr); err != nil {
		return err
	}
	return nil
}

// 255 is common limit for file names.
// -5 to account for .json extension
const maxPathLength = 250

// returns tempfir containing backup contents, map[sha256]longNames, error
func (e *Extractor) readBackup(tarRdr *tar.Reader) (string, map[string]string, error) {
	dir, err := e.fs.TempDir("", "")
	if err != nil {
		e.log.Infof("error creating temp dir: %v", err)
		return "", nil, err
	}
	longNames := make(map[string]string)
	for {
		header, err := tarRdr.Next()

		if err == io.EOF {
			break
		}
		if err != nil {
			e.log.Infof("error reading tar: %v", err)
			return "", nil, err
		}

		target := filepath.Join(dir, header.Name) //nolint:gosec // Internal usage. No need to check.
		// If the target is longer than maxPathLength, we'll use the sha256 of the header name as the filename.
		// https://github.com/vmware-tanzu/velero/issues/8434
		if len(target) > maxPathLength {
			shortSha256Name := fmt.Sprintf("%x", sha256.Sum256([]byte(header.Name))) // sha256 name is 64 characters
			longNames[shortSha256Name] = header.Name
			target = filepath.Join(dir, shortSha256Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			err := e.fs.MkdirAll(target, header.FileInfo().Mode())
			if err != nil {
				e.log.Infof("mkdirall error: %v", err)
				return "", nil, err
			}

		case tar.TypeReg:
			// make sure we have the directory created
			err := e.fs.MkdirAll(filepath.Dir(target), header.FileInfo().Mode())
			if err != nil {
				e.log.Infof("mkdirall error: %v", err)
				return "", nil, err
			}

			// create the file
			if err := e.writeFile(target, tarRdr); err != nil {
				e.log.Infof("error copying: %v", err)
				return "", nil, err
			}
		}
	}
	return dir, longNames, nil
}
