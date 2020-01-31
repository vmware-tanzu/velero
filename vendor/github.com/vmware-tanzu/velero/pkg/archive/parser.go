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

package archive

import (
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
)

// Parser traverses an extracted archive on disk to validate
// it and provide a helpful representation of it to consumers.
type Parser struct {
	log logrus.FieldLogger
	fs  filesystem.Interface
}

// ResourceItems contains the collection of items of a given resource type
// within a backup, grouped by namespace (or empty string for cluster-scoped
// resources).
type ResourceItems struct {
	// GroupResource is API group and resource name,
	// formatted as "resource.group". For the "core"
	// API group, the ".group" suffix is omitted.
	GroupResource string

	// ItemsByNamespace is a map from namespace (or empty string
	// for cluster-scoped resources) to a list of individual item
	// names contained in the archive. Item names **do not** include
	// the file extension.
	ItemsByNamespace map[string][]string
}

// NewParser constructs a Parser.
func NewParser(log logrus.FieldLogger, fs filesystem.Interface) *Parser {
	return &Parser{
		log: log,
		fs:  fs,
	}
}

// Parse reads an extracted backup on the file system and returns
// a structured catalog of the resources and items contained within it.
func (p *Parser) Parse(dir string) (map[string]*ResourceItems, error) {
	// ensure top-level "resources" directory exists, and read subdirectories
	// of it, where each one is expected to correspond to a resource.
	resourcesDir := filepath.Join(dir, velerov1api.ResourcesDir)
	exists, err := p.fs.DirExists(resourcesDir)
	if err != nil {
		return nil, errors.Wrapf(err, "error checking for existence of directory %q", strings.TrimPrefix(resourcesDir, dir+"/"))
	}
	if !exists {
		return nil, errors.Errorf("directory %q does not exist", strings.TrimPrefix(resourcesDir, dir+"/"))
	}

	resourceDirs, err := p.fs.ReadDir(resourcesDir)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading contents of directory %q", strings.TrimPrefix(resourcesDir, dir+"/"))
	}

	// loop through each subdirectory (one per resource) and assemble
	// catalog of items within it.
	resources := map[string]*ResourceItems{}
	for _, resourceDir := range resourceDirs {
		if !resourceDir.IsDir() {
			p.log.Warnf("Ignoring unexpected file %q in directory %q", resourceDir.Name(), strings.TrimPrefix(resourcesDir, dir+"/"))
			continue
		}

		resourceItems := &ResourceItems{
			GroupResource:    resourceDir.Name(),
			ItemsByNamespace: map[string][]string{},
		}

		// check for existence of a "cluster" subdirectory containing cluster-scoped
		// instances of this resource, and read its contents if it exists.
		clusterScopedDir := filepath.Join(resourcesDir, resourceDir.Name(), velerov1api.ClusterScopedDir)
		exists, err := p.fs.DirExists(clusterScopedDir)
		if err != nil {
			return nil, errors.Wrapf(err, "error checking for existence of directory %q", strings.TrimPrefix(clusterScopedDir, dir+"/"))
		}
		if exists {
			items, err := p.getResourceItemsForScope(clusterScopedDir, dir)
			if err != nil {
				return nil, err
			}

			if len(items) > 0 {
				resourceItems.ItemsByNamespace[""] = items
			}
		}

		// check for existence of a "namespaces" subdirectory containing further subdirectories,
		// one per namespace, and read its contents if it exists.
		namespaceScopedDir := filepath.Join(resourcesDir, resourceDir.Name(), velerov1api.NamespaceScopedDir)
		exists, err = p.fs.DirExists(namespaceScopedDir)
		if err != nil {
			return nil, errors.Wrapf(err, "error checking for existence of directory %q", strings.TrimPrefix(namespaceScopedDir, dir+"/"))
		}
		if exists {
			namespaceDirs, err := p.fs.ReadDir(namespaceScopedDir)
			if err != nil {
				return nil, errors.Wrapf(err, "error reading contents of directory %q", strings.TrimPrefix(namespaceScopedDir, dir+"/"))
			}

			for _, namespaceDir := range namespaceDirs {
				if !namespaceDir.IsDir() {
					p.log.Warnf("Ignoring unexpected file %q in directory %q", namespaceDir.Name(), strings.TrimPrefix(namespaceScopedDir, dir+"/"))
					continue
				}

				items, err := p.getResourceItemsForScope(filepath.Join(namespaceScopedDir, namespaceDir.Name()), dir)
				if err != nil {
					return nil, err
				}

				if len(items) > 0 {
					resourceItems.ItemsByNamespace[namespaceDir.Name()] = items
				}
			}
		}

		resources[resourceDir.Name()] = resourceItems
	}

	return resources, nil
}

// getResourceItemsForScope returns the list of items with a namespace or
// cluster-scoped subdirectory for a specific resource.
func (p *Parser) getResourceItemsForScope(dir, archiveRootDir string) ([]string, error) {
	files, err := p.fs.ReadDir(dir)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading contents of directory %q", strings.TrimPrefix(dir, archiveRootDir+"/"))
	}

	var items []string
	for _, file := range files {
		if file.IsDir() {
			p.log.Warnf("Ignoring unexpected subdirectory %q in directory %q", file.Name(), strings.TrimPrefix(dir, archiveRootDir+"/"))
			continue
		}

		items = append(items, strings.TrimSuffix(file.Name(), ".json"))
	}

	return items, nil
}
