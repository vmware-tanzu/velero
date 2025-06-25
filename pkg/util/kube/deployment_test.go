package kube

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1api "k8s.io/api/apps/v1"
	corev1api "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/vmware-tanzu/velero/pkg/util/velero"
)

type simpleErrMockClient struct {
	ctrlclient.WithWatch
	getError error
}

func (m *simpleErrMockClient) Get(ctx context.Context, key ctrlclient.ObjectKey, obj ctrlclient.Object, opts ...ctrlclient.GetOption) error {
	if m.getError != nil {
		return m.getError
	}
	return m.WithWatch.Get(ctx, key, obj, opts...)
}

func TestGetVeleroDeployment(t *testing.T) {
	const (
		defaultVeleroName   = "velero"
		veleroContainerName = defaultVeleroName
		customVeleroName    = "custom-velero"

		namespace = "velero-ns"
	)

	ctx := context.TODO()

	scheme := runtime.NewScheme()
	require.NoError(t, appsv1api.AddToScheme(scheme))
	require.NoError(t, corev1api.AddToScheme(scheme))

	tests := []struct {
		name               string
		objects            []ctrlclient.Object
		clientWrapper      func(ctrlclient.WithWatch) ctrlclient.WithWatch
		expectedError      string
		expectedDeployName string
	}{
		{
			name: "found by name",
			objects: []ctrlclient.Object{
				&appsv1api.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      defaultVeleroName,
						Namespace: namespace,
					},
					Spec: appsv1api.DeploymentSpec{
						Template: corev1api.PodTemplateSpec{
							Spec: corev1api.PodSpec{
								Containers: []corev1api.Container{{Name: veleroContainerName}},
							},
						},
					},
				},
			},
			expectedDeployName: defaultVeleroName,
		},
		{
			name: "get returns non-notfound error",
			clientWrapper: func(c ctrlclient.WithWatch) ctrlclient.WithWatch {
				return &simpleErrMockClient{WithWatch: c, getError: apierrors.NewUnauthorized("foo")}
			},
			expectedError: "foo",
		},
		{
			name:          "fallback to label - none found",
			expectedError: "could not find velero deployment",
		},
		{
			name: "fallback to label - found",
			objects: []ctrlclient.Object{
				&appsv1api.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      customVeleroName,
						Namespace: namespace,
						Labels:    velero.Labels(),
					},
					Spec: appsv1api.DeploymentSpec{
						Template: corev1api.PodTemplateSpec{
							Spec: corev1api.PodSpec{
								Containers: []corev1api.Container{{Name: veleroContainerName}},
							},
						},
					},
				},
			},
			expectedDeployName: customVeleroName,
		},
		{
			name: "deployment found but container missing",
			objects: []ctrlclient.Object{
				&appsv1api.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      defaultVeleroName,
						Namespace: namespace,
					},
					Spec: appsv1api.DeploymentSpec{
						Template: corev1api.PodTemplateSpec{
							Spec: corev1api.PodSpec{
								Containers: []corev1api.Container{{Name: "wrong"}},
							},
						},
					},
				},
			},
			expectedError: "velero deployment not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(scheme)
			if tt.objects != nil {
				builder = builder.WithObjects(tt.objects...)
			}

			client := builder.Build()
			if tt.clientWrapper != nil {
				client = tt.clientWrapper(client)
			}

			dep, err := GetVeleroDeployment(ctx, client, namespace)
			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedDeployName, dep.Name)
			}
		})
	}
}
