package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/rest"
)

type ResultWriter struct {
	workdir   string
	writeLogs bool
	restApi   rest.Interface
}

func NewResultWriter(workdir, what string, restApi rest.Interface) (*ResultWriter, error) {
	var err error
	workdir = filepath.Join(workdir, BaseDirname)
	if err := os.MkdirAll(workdir, 0744); err != nil && !os.IsExist(err) {
		return nil, err
	}

	writeLogs := what == "logs" || what == "all"
	return &ResultWriter{
		workdir:   workdir,
		writeLogs: writeLogs,
		restApi:   restApi,
	}, err
}

func (w *ResultWriter) GetResultDir() string {
	return w.workdir
}

func (w *ResultWriter) Write(ctx context.Context, searchResults []SearchResult) error {
	if len(searchResults) == 0 {
		return fmt.Errorf("cannot write empty (or nil) search result")
	}

	// each result represents a list of searched item
	// write each list in a namespaced location in working dir
	for _, result := range searchResults {
		objWriter := ObjectWriter{
			writeDir: w.workdir,
		}
		writeDir, err := objWriter.Write(result)
		if err != nil {
			return err
		}

		if w.writeLogs && result.ListKind == "PodList" {
			if len(result.List.Items) == 0 {
				continue
			}
			for _, podItem := range result.List.Items {
				logDir := filepath.Join(writeDir, podItem.GetName())
				if err := os.MkdirAll(logDir, 0744); err != nil && !os.IsExist(err) {
					return fmt.Errorf("failed to create pod log dir: %s", err)
				}

				containers, err := GetContainers(podItem)
				if err != nil {
					return err
				}
				for _, containerLogger := range containers {
					reader, err := containerLogger.Fetch(ctx, w.restApi)
					if err != nil {
						return err
					}
					err = containerLogger.Write(reader, logDir)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}
