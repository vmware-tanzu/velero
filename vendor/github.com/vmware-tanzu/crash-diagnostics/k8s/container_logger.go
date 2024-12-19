package k8s

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type ContainerLogsImpl struct {
	namespace string
	podName   string
	container corev1.Container
}

func NewContainerLogger(namespace, podName string, container corev1.Container) ContainerLogsImpl {
	return ContainerLogsImpl{
		namespace: namespace,
		podName:   podName,
		container: container,
	}
}

func (c ContainerLogsImpl) Fetch(ctx context.Context, restApi rest.Interface) (io.ReadCloser, error) {
	opts := &corev1.PodLogOptions{Container: c.container.Name}
	req := restApi.Get().Namespace(c.namespace).Name(c.podName).Resource("pods").SubResource("log").VersionedParams(opts, scheme.ParameterCodec)
	stream, err := req.Stream(ctx)
	if err != nil {
		err = errors.Wrap(err, "failed to create container log stream")
	}
	return stream, err
}

func (c ContainerLogsImpl) Write(reader io.ReadCloser, rootDir string) error {
	containerLogDir := filepath.Join(rootDir, c.container.Name)
	if err := os.MkdirAll(containerLogDir, 0744); err != nil && !os.IsExist(err) {
		return fmt.Errorf("error creating container log dir: %s", err)
	}

	path := filepath.Join(containerLogDir, fmt.Sprintf("%s.log", c.container.Name))
	logrus.Debugf("Writing pod container log %s", path)

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	defer reader.Close()
	if _, err := io.Copy(file, reader); err != nil {
		cpErr := fmt.Errorf("failed to copy container log:\n%s", err)
		if wErr := writeError(cpErr, file); wErr != nil {
			return fmt.Errorf("failed to write previous err [%s] to file: %s", err, wErr)
		}
		return err
	}
	return nil
}
