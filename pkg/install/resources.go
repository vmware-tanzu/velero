package install

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1beta1 "k8s.io/api/rbac/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func labels() map[string]string {
	return map[string]string{
		"component": "ark",
	}
}

func objectMeta(namespace, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels:    labels(),
	}
}

func ServiceAccount(namespace string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: objectMeta(namespace, "ark"),
	}
}

func ClusterRoleBinding(namespace string) *rbacv1beta1.ClusterRoleBinding {
	return &rbacv1beta1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ark",
		},
		Subjects: []rbacv1beta1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: namespace,
				Name:      "ark",
			},
		},
		RoleRef: rbacv1beta1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
}

func Namespace(namespace string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
}
