package kube

import (
	"context"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/cluster-api/controllers/remote"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewClusterClients(ctx context.Context, c client.Client, cluster client.ObjectKey) (*kubernetes.Clientset, dynamic.Interface, error) {
	restConfig, err := remote.RESTConfig(ctx, c, cluster)
	if err != nil {
		return nil, nil, err
	}
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, err
	}
	return kubeClient, dynamicClient, nil
}
