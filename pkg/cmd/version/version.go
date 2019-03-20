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

package version

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	velerov1api "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/buildinfo"
	"github.com/heptio/velero/pkg/client"
	"github.com/heptio/velero/pkg/cmd"
	velerov1client "github.com/heptio/velero/pkg/generated/clientset/versioned/typed/velero/v1"
	"github.com/heptio/velero/pkg/serverstatusrequest"
)

func NewCommand(f client.Factory) *cobra.Command {
	clientOnly := false
	serverStatusGetter := &defaultServerStatusGetter{
		namespace: f.Namespace(),
		timeout:   5 * time.Second,
	}

	c := &cobra.Command{
		Use:   "version",
		Short: "Print the velero version and associated image",
		Run: func(c *cobra.Command, args []string) {
			var veleroClient velerov1client.ServerStatusRequestsGetter

			if !clientOnly {
				client, err := f.Client()
				cmd.CheckError(err)

				veleroClient = client.VeleroV1()
			}

			printVersion(os.Stdout, clientOnly, veleroClient, serverStatusGetter)
		},
	}

	c.Flags().DurationVar(&serverStatusGetter.timeout, "timeout", serverStatusGetter.timeout, "maximum time to wait for server version to be reported")
	c.Flags().BoolVar(&clientOnly, "client-only", clientOnly, "only get velero client version, not server version")

	return c
}

func printVersion(w io.Writer, clientOnly bool, client velerov1client.ServerStatusRequestsGetter, serverStatusGetter serverStatusGetter) {
	fmt.Fprintln(w, "Client:")
	fmt.Fprintf(w, "\tVersion: %s\n", buildinfo.Version)
	fmt.Fprintf(w, "\tGit commit: %s\n", buildinfo.FormattedGitSHA())

	if clientOnly {
		return
	}

	serverStatus, err := serverStatusGetter.getServerStatus(client)
	if err != nil {
		fmt.Fprintf(w, "<error getting server version: %s>\n", err)
		return
	}

	fmt.Fprintln(w, "Server:")
	fmt.Fprintf(w, "\tVersion: %s\n", serverStatus.Status.ServerVersion)
}

type serverStatusGetter interface {
	getServerStatus(client velerov1client.ServerStatusRequestsGetter) (*velerov1api.ServerStatusRequest, error)
}

type defaultServerStatusGetter struct {
	namespace string
	timeout   time.Duration
}

func (g *defaultServerStatusGetter) getServerStatus(client velerov1client.ServerStatusRequestsGetter) (*velerov1api.ServerStatusRequest, error) {
	req := serverstatusrequest.NewBuilder().Namespace(g.namespace).GenerateName("velero-cli-").Build()

	created, err := client.ServerStatusRequests(g.namespace).Create(req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer client.ServerStatusRequests(g.namespace).Delete(created.Name, nil)

	listOptions := metav1.ListOptions{
		// TODO: once the minimum supported Kubernetes version is v1.9.0, uncomment the following line.
		// See http://issue.k8s.io/51046 for details.
		//FieldSelector:   "metadata.name=" + req.Name
		ResourceVersion: created.ResourceVersion,
	}
	watcher, err := client.ServerStatusRequests(g.namespace).Watch(listOptions)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer watcher.Stop()

	expired := time.NewTimer(g.timeout)
	defer expired.Stop()

Loop:
	for {
		select {
		case <-expired.C:
			return nil, errors.New("timed out waiting for server status request to be processed")
		case e := <-watcher.ResultChan():
			updated, ok := e.Object.(*velerov1api.ServerStatusRequest)
			if !ok {
				return nil, errors.Errorf("unexpected type %T", e.Object)
			}

			// TODO: once the minimum supported Kubernetes version is v1.9.0, remove the following check.
			// See http://issue.k8s.io/51046 for details.
			if updated.Name != created.Name {
				continue
			}

			switch e.Type {
			case watch.Deleted:
				return nil, errors.New("server status request was unexpectedly deleted")
			case watch.Modified:
				if updated.Status.Phase == velerov1api.ServerStatusRequestPhaseProcessed {
					req = updated
					break Loop
				}
			}
		}
	}

	return req, nil
}
