/*
Copyright 2017 the Velero contributors.

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

package downloadrequest

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	core "k8s.io/client-go/testing"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/fake"
)

func TestStream(t *testing.T) {
	tests := []struct {
		name          string
		kind          v1.DownloadTargetKind
		timeout       time.Duration
		createError   error
		watchError    error
		watchAdds     []runtime.Object
		watchModifies []runtime.Object
		watchDeletes  []runtime.Object
		updateWithURL bool
		statusCode    int
		body          string
		deleteError   error
		expectedError string
	}{
		{
			name:          "error creating req",
			createError:   errors.New("foo"),
			kind:          v1.DownloadTargetKindBackupLog,
			expectedError: "foo",
		},
		{
			name:          "error creating watch",
			watchError:    errors.New("bar"),
			kind:          v1.DownloadTargetKindBackupLog,
			expectedError: "bar",
		},
		{
			name:          "timed out",
			kind:          v1.DownloadTargetKindBackupLog,
			timeout:       time.Millisecond,
			expectedError: "timed out waiting for download URL",
		},
		{
			name:          "unexpected watch type",
			kind:          v1.DownloadTargetKindBackupLog,
			watchAdds:     []runtime.Object{&v1.Backup{}},
			expectedError: "unexpected type *v1.Backup",
		},
		{
			name: "other requests added/updated/deleted first",
			kind: v1.DownloadTargetKindBackupLog,
			watchAdds: []runtime.Object{
				newDownloadRequest("foo").DownloadRequest,
			},
			watchModifies: []runtime.Object{
				newDownloadRequest("foo").DownloadRequest,
			},
			watchDeletes: []runtime.Object{
				newDownloadRequest("foo").DownloadRequest,
			},
			updateWithURL: true,
			statusCode:    http.StatusOK,
			body:          "download body",
		},
		{
			name:          "http error",
			kind:          v1.DownloadTargetKindBackupLog,
			updateWithURL: true,
			statusCode:    http.StatusInternalServerError,
			body:          "some error",
			expectedError: "request failed: some error",
		},
	}

	const testTimeout = 30 * time.Second

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()

			created := make(chan *v1.DownloadRequest, 1)
			client.PrependReactor("create", "downloadrequests", func(action core.Action) (bool, runtime.Object, error) {
				createAction := action.(core.CreateAction)
				created <- createAction.GetObject().(*v1.DownloadRequest)
				return true, createAction.GetObject(), test.createError
			})

			fakeWatch := watch.NewFake()
			client.PrependWatchReactor("downloadrequests", core.DefaultWatchReactor(fakeWatch, test.watchError))

			deleted := make(chan string, 1)
			client.PrependReactor("delete", "downloadrequests", func(action core.Action) (bool, runtime.Object, error) {
				deleteAction := action.(core.DeleteAction)
				deleted <- deleteAction.GetName()
				return true, nil, test.deleteError
			})

			timeout := test.timeout
			if timeout == 0 {
				timeout = testTimeout
			}

			var server *httptest.Server
			var url string
			if test.updateWithURL {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					w.WriteHeader(test.statusCode)
					if test.statusCode == http.StatusOK {
						gzipWriter := gzip.NewWriter(w)
						fmt.Fprintf(gzipWriter, test.body)
						gzipWriter.Close()
						return
					}
					fmt.Fprintf(w, test.body)
				}))
				defer server.Close()
				url = server.URL
			}

			output := new(bytes.Buffer)
			errCh := make(chan error)
			go func() {
				err := Stream(client.VeleroV1(), "namespace", "name", test.kind, output, timeout, false, "")
				errCh <- err
			}()

			for i := range test.watchAdds {
				fakeWatch.Add(test.watchAdds[i])
			}
			for i := range test.watchModifies {
				fakeWatch.Modify(test.watchModifies[i])
			}
			for i := range test.watchDeletes {
				fakeWatch.Delete(test.watchDeletes[i])
			}

			var createdName string
			if test.updateWithURL {
				select {
				case r := <-created:
					createdName = r.Name
					r.Status.DownloadURL = url
					fakeWatch.Modify(r)
				case <-time.After(testTimeout):
					t.Fatalf("created object not received")
				}

			}

			var err error
			select {
			case err = <-errCh:
			case <-time.After(testTimeout):
				t.Fatal("test timed out")
			}

			if test.expectedError != "" {
				require.EqualError(t, err, test.expectedError)
				return
			}

			require.NoError(t, err)

			if test.statusCode != http.StatusOK {
				assert.EqualError(t, err, "request failed: "+test.body)
				return
			}

			assert.Equal(t, test.body, output.String())

			select {
			case name := <-deleted:
				assert.Equal(t, createdName, name)
			default:
				t.Fatal("download request was not deleted")
			}
		})
	}
}

type downloadRequest struct {
	*v1.DownloadRequest
}

func newDownloadRequest(name string) *downloadRequest {
	return &downloadRequest{
		DownloadRequest: &v1.DownloadRequest{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: v1.DefaultNamespace,
				Name:      name,
			},
		},
	}
}
