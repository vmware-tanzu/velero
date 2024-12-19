package k8s

import (
	"context"
	"fmt"
	"io"

	"k8s.io/client-go/rest"
)

const BaseDirname = "kubecapture"

type Container interface {
	Fetch(context.Context, rest.Interface) (io.ReadCloser, error)
	Write(io.ReadCloser, string) error
}

func writeError(errStr error, w io.Writer) error {
	_, err := fmt.Fprintln(w, errStr.Error())
	return err
}
