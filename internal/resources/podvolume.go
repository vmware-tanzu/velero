package podvolume

import (
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

func PodLister() {
	lister := corev1listers.NewPodLister(podInformer.GetIndexer())
	return 
}