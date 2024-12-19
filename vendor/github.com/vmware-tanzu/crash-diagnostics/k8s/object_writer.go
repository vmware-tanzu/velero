package k8s

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/cli-runtime/pkg/printers"
)

type ObjectWriter struct {
	writeDir string
}

func (w ObjectWriter) Write(result SearchResult) (string, error) {
	// namespaced on group and version to avoid overwrites
	grp := func() string {
		if result.GroupVersionResource.Group == "" {
			return "core"
		}
		return result.GroupVersionResource.Group
	}()

	w.writeDir = filepath.Join(w.writeDir, fmt.Sprintf("%s_%s", grp, result.GroupVersionResource.Version))

	// add resource namespace if needed
	if result.Namespaced {
		w.writeDir = filepath.Join(w.writeDir, result.Namespace)
	}

	if err := os.MkdirAll(w.writeDir, 0744); err != nil && !os.IsExist(err) {
		return "", fmt.Errorf("failed to create search result dir: %s", err)
	}

	now := time.Now().Format("200601021504.0000")
	path := filepath.Join(w.writeDir, fmt.Sprintf("%s-%s.json", result.ResourceName, now))

	file, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	logrus.Debugf("objectWriter: saving %s search results to: %s", result.ResourceName, path)

	printer := new(printers.JSONPrinter)
	if err := printer.PrintObj(result.List, file); err != nil {
		if wErr := writeError(err, file); wErr != nil {
			return "", fmt.Errorf("objectWriter: failed to write previous err [%s] to file: %s", err, wErr)
		}
		return "", err
	}
	return w.writeDir, nil
}
