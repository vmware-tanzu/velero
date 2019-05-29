package serverstatus

import (
	"time"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	velerov1api "github.com/heptio/velero/pkg/apis/velero/v1"
	velerov1client "github.com/heptio/velero/pkg/generated/clientset/versioned/typed/velero/v1"
	"github.com/heptio/velero/pkg/serverstatusrequest"
)

type ServerStatusGetter interface {
	GetServerStatus(client velerov1client.ServerStatusRequestsGetter) (*velerov1api.ServerStatusRequest, error)
}

type DefaultServerStatusGetter struct {
	Namespace string
	Timeout   time.Duration
}

func (g *DefaultServerStatusGetter) GetServerStatus(client velerov1client.ServerStatusRequestsGetter) (*velerov1api.ServerStatusRequest, error) {
	req := serverstatusrequest.NewBuilder().Namespace(g.Namespace).GenerateName("velero-cli-").Build()

	created, err := client.ServerStatusRequests(g.Namespace).Create(req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer client.ServerStatusRequests(g.Namespace).Delete(created.Name, nil)

	listOptions := metav1.ListOptions{
		// TODO: once the minimum supported Kubernetes version is v1.9.0, uncomment the following line.
		// See http://issue.k8s.io/51046 for details.
		//FieldSelector:   "metadata.name=" + req.Name
		ResourceVersion: created.ResourceVersion,
	}
	watcher, err := client.ServerStatusRequests(g.Namespace).Watch(listOptions)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer watcher.Stop()

	expired := time.NewTimer(g.Timeout)
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
