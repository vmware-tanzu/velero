/*
Copyright the Velero contributors.

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

package kube

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

func TestStart(t *testing.T) {
	require.Nil(t, velerov1.AddToScheme(scheme.Scheme))

	ctx, cancelFunc := context.WithCancel(context.TODO())
	client := (&fake.ClientBuilder{}).Build()
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultItemBasedRateLimiter())
	source := NewPeriodicalEnqueueSource(logrus.WithContext(ctx), client, &velerov1.ScheduleList{}, 1*time.Second)

	require.Nil(t, source.Start(ctx, nil, queue))

	// no resources
	time.Sleep(1 * time.Second)
	require.Equal(t, queue.Len(), 0)

	// contain one resource
	require.Nil(t, client.Create(ctx, &velerov1.Schedule{
		ObjectMeta: metav1.ObjectMeta{
			Name: "schedule",
		},
	}))
	time.Sleep(2 * time.Second)
	require.Equal(t, queue.Len(), 1)

	// context canceled, the enqueue source shouldn't run anymore
	item, _ := queue.Get()
	queue.Forget(item)
	require.Equal(t, queue.Len(), 0)
	cancelFunc()
	time.Sleep(2 * time.Second)
	require.Equal(t, queue.Len(), 0)
}
