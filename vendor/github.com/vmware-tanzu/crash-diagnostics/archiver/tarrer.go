// Copyright (c) 2019 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package archiver

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

// Tar compresses the file sources specified by paths into a single
// tarball specified by tarName.
func Tar(tarName string, paths ...string) (err error) {
	logrus.Debugf("Archiving %v in %s", paths, tarName)
	tarFile, err := os.Create(tarName)
	if err != nil {
		return err
	}
	defer func() {
		err = tarFile.Close()
	}()

	absTar, err := filepath.Abs(tarName)
	if err != nil {
		return err
	}

	// enable compression if file ends in .gz
	tw := tar.NewWriter(tarFile)
	if strings.HasSuffix(tarName, ".gz") || strings.HasSuffix(tarName, ".gzip") {
		gz := gzip.NewWriter(tarFile)
		defer gz.Close()
		tw = tar.NewWriter(gz)
	}
	defer tw.Close()

	// walk each path and add encountered file to tar
	for _, path := range paths {
		// validate path
		path = filepath.Clean(path)
		absPath, err := filepath.Abs(path)
		if err != nil {
			logrus.Error(err)
			continue
		}
		if absPath == absTar {
			logrus.Errorf("Tar file %s cannot be the source, skipping path", tarName)
			continue
		}
		if absPath == filepath.Dir(absTar) {
			logrus.Errorf("Tar file %s cannot be in source %s, skipping path", tarName, absPath)
			continue
		}

		// build tar
		err = filepath.Walk(path, func(file string, finfo os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			relFilePath := file
			if filepath.IsAbs(path) {
				relFilePath, err = filepath.Rel("/", file)
				if err != nil {
					return err
				}
			}

			hdr, err := tar.FileInfoHeader(finfo, finfo.Name())
			if err != nil {
				return err
			}
			// ensure header has relative file path
			hdr.Name = relFilePath
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			if finfo.Mode().IsDir() {
				return nil
			}

			// add file to tar
			srcFile, err := os.Open(file)
			if err != nil {
				return err
			}
			defer srcFile.Close()
			_, err = io.Copy(tw, srcFile)
			if err != nil {
				return err
			}
			logrus.Debugf("Archived %s", file)
			return nil
		})
		if err != nil {
			logrus.Errorf("failed to add %s to archive %s: %v", path, tarName, err)
		}
	}
	return nil
}
