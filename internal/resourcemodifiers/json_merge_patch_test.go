package resourcemodifiers

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

func TestJsonMergePatchFailure(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "patch with bad yaml",
			data: []byte("a: b:"),
		},
		{
			name: "patch with bad json",
			data: []byte(`{"a"::1}`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			err := clientgoscheme.AddToScheme(scheme)
			assert.NoError(t, err)
			pt := &JSONMergePatcher{
				patches: []JSONMergePatch{{PatchData: tt.data}},
			}

			u := &unstructured.Unstructured{}
			_, err = pt.Patch(u, logrus.New())
			assert.Error(t, err)
		})
	}
}
