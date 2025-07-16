package exposer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1api "k8s.io/api/apps/v1"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	clientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/datapath"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
)

func TestPodVolumeExpose(t *testing.T) {
	backup := &velerov1.Backup{
		TypeMeta: metav1.TypeMeta{
			APIVersion: velerov1.SchemeGroupVersion.String(),
			Kind:       "Backup",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: velerov1.DefaultNamespace,
			Name:      "fake-backup",
			UID:       "fake-uid",
		},
	}

	podWithNoNode := builder.ForPod("fake-ns", "fake-client-pod").Result()
	podWithNode := builder.ForPod("fake-ns", "fake-client-pod").NodeName("fake-node").Result()

	node := builder.ForNode("fake-node").Result()

	daemonSet := &appsv1api.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "velero",
			Name:      "node-agent",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "DaemonSet",
			APIVersion: appsv1api.SchemeGroupVersion.String(),
		},
		Spec: appsv1api.DaemonSetSpec{
			Template: corev1api.PodTemplateSpec{
				Spec: corev1api.PodSpec{
					Containers: []corev1api.Container{
						{
							Name: "node-agent",
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name                         string
		snapshotClientObj            []runtime.Object
		kubeClientObj                []runtime.Object
		ownerBackup                  *velerov1.Backup
		exposeParam                  PodVolumeExposeParam
		funcGetPodVolumeHostPath     func(context.Context, *corev1api.Pod, string, kubernetes.Interface, filesystem.Interface, logrus.FieldLogger) (datapath.AccessPoint, error)
		funcExtractPodVolumeHostPath func(context.Context, string, kubernetes.Interface, string, string) (string, error)
		err                          string
	}{
		{
			name:        "get client pod fail",
			ownerBackup: backup,
			exposeParam: PodVolumeExposeParam{
				ClientNamespace: "fake-ns",
				ClientPodName:   "fake-client-pod",
			},
			err: "error getting client pod fake-client-pod: pods \"fake-client-pod\" not found",
		},
		{
			name:        "client pod with no node name",
			ownerBackup: backup,
			exposeParam: PodVolumeExposeParam{
				ClientNamespace: "fake-ns",
				ClientPodName:   "fake-client-pod",
			},
			kubeClientObj: []runtime.Object{
				podWithNoNode,
			},
			err: "client pod fake-client-pod doesn't have a node name",
		},
		{
			name:        "get node os fail",
			ownerBackup: backup,
			exposeParam: PodVolumeExposeParam{
				ClientNamespace: "fake-ns",
				ClientPodName:   "fake-client-pod",
			},
			kubeClientObj: []runtime.Object{
				podWithNode,
			},
			err: "error getting OS for node fake-node: error getting node fake-node: nodes \"fake-node\" not found",
		},
		{
			name:        "get pod volume path fail",
			ownerBackup: backup,
			exposeParam: PodVolumeExposeParam{
				ClientNamespace: "fake-ns",
				ClientPodName:   "fake-client-pod",
				ClientPodVolume: "fake-client-volume",
			},
			kubeClientObj: []runtime.Object{
				podWithNode,
				node,
			},
			funcGetPodVolumeHostPath: func(context.Context, *corev1api.Pod, string, kubernetes.Interface, filesystem.Interface, logrus.FieldLogger) (datapath.AccessPoint, error) {
				return datapath.AccessPoint{}, errors.New("fake-get-pod-volume-path-error")
			},
			err: "error to get pod volume path: fake-get-pod-volume-path-error",
		},
		{
			name:        "extract pod volume path fail",
			ownerBackup: backup,
			exposeParam: PodVolumeExposeParam{
				ClientNamespace: "fake-ns",
				ClientPodName:   "fake-client-pod",
				ClientPodVolume: "fake-client-volume",
			},
			kubeClientObj: []runtime.Object{
				podWithNode,
				node,
			},
			funcGetPodVolumeHostPath: func(context.Context, *corev1api.Pod, string, kubernetes.Interface, filesystem.Interface, logrus.FieldLogger) (datapath.AccessPoint, error) {
				return datapath.AccessPoint{
					ByPath: "/var/lib/kubelet/pods/pod-id-xxx/volumes/kubernetes.io~csi/pvc-id-xxx/mount",
				}, nil
			},
			funcExtractPodVolumeHostPath: func(context.Context, string, kubernetes.Interface, string, string) (string, error) {
				return "", errors.New("fake-extract-error")
			},
			err: "error to extract pod volume path: fake-extract-error",
		},
		{
			name:        "create hosting pod fail",
			ownerBackup: backup,
			exposeParam: PodVolumeExposeParam{
				ClientNamespace: "fake-ns",
				ClientPodName:   "fake-client-pod",
				ClientPodVolume: "fake-client-volume",
			},
			kubeClientObj: []runtime.Object{
				podWithNode,
				node,
			},
			funcGetPodVolumeHostPath: func(context.Context, *corev1api.Pod, string, kubernetes.Interface, filesystem.Interface, logrus.FieldLogger) (datapath.AccessPoint, error) {
				return datapath.AccessPoint{
					ByPath: "/host_pods/pod-id-xxx/volumes/kubernetes.io~csi/pvc-id-xxx/mount",
				}, nil
			},
			funcExtractPodVolumeHostPath: func(context.Context, string, kubernetes.Interface, string, string) (string, error) {
				return "/var/lib/kubelet/pods/pod-id-xxx/volumes/kubernetes.io~csi/pvc-id-xxx/mount", nil
			},
			err: "error to create hosting pod: error to get inherited pod info from node-agent: error to get node-agent pod template: error to get node-agent daemonset: daemonsets.apps \"node-agent\" not found",
		},
		{
			name:        "succeed",
			ownerBackup: backup,
			exposeParam: PodVolumeExposeParam{
				ClientNamespace: "fake-ns",
				ClientPodName:   "fake-client-pod",
				ClientPodVolume: "fake-client-volume",
			},
			kubeClientObj: []runtime.Object{
				podWithNode,
				node,
				daemonSet,
			},
			funcGetPodVolumeHostPath: func(context.Context, *corev1api.Pod, string, kubernetes.Interface, filesystem.Interface, logrus.FieldLogger) (datapath.AccessPoint, error) {
				return datapath.AccessPoint{
					ByPath: "/host_pods/pod-id-xxx/volumes/kubernetes.io~csi/pvc-id-xxx/mount",
				}, nil
			},
			funcExtractPodVolumeHostPath: func(context.Context, string, kubernetes.Interface, string, string) (string, error) {
				return "/var/lib/kubelet/pods/pod-id-xxx/volumes/kubernetes.io~csi/pvc-id-xxx/mount", nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.kubeClientObj...)

			exposer := podVolumeExposer{
				kubeClient: fakeKubeClient,
				log:        velerotest.NewLogger(),
			}

			var ownerObject corev1api.ObjectReference
			if test.ownerBackup != nil {
				ownerObject = corev1api.ObjectReference{
					Kind:       test.ownerBackup.Kind,
					Namespace:  test.ownerBackup.Namespace,
					Name:       test.ownerBackup.Name,
					UID:        test.ownerBackup.UID,
					APIVersion: test.ownerBackup.APIVersion,
				}
			}

			if test.funcGetPodVolumeHostPath != nil {
				getPodVolumeHostPath = test.funcGetPodVolumeHostPath
			}

			if test.funcExtractPodVolumeHostPath != nil {
				extractPodVolumeHostPath = test.funcExtractPodVolumeHostPath
			}

			err := exposer.Expose(context.Background(), ownerObject, test.exposeParam)
			if err == nil {
				require.NoError(t, err)

				_, err = exposer.kubeClient.CoreV1().Pods(ownerObject.Namespace).Get(context.Background(), ownerObject.Name, metav1.GetOptions{})
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.err)
			}
		})
	}
}

func TestGetPodVolumeExpose(t *testing.T) {
	backup := &velerov1.Backup{
		TypeMeta: metav1.TypeMeta{
			APIVersion: velerov1.SchemeGroupVersion.String(),
			Kind:       "Backup",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: velerov1.DefaultNamespace,
			Name:      "fake-backup",
			UID:       "fake-uid",
		},
	}

	backupPodNotRunning := builder.ForPod(backup.Namespace, backup.Name).Result()
	backupPodRunning := builder.ForPod(backup.Namespace, backup.Name).Phase(corev1api.PodRunning).Result()

	scheme := runtime.NewScheme()
	corev1api.AddToScheme(scheme)

	tests := []struct {
		name           string
		kubeClientObj  []runtime.Object
		ownerBackup    *velerov1.Backup
		nodeName       string
		Timeout        time.Duration
		err            string
		expectedResult *ExposeResult
	}{
		{
			name:        "backup pod is not found",
			ownerBackup: backup,
			nodeName:    "fake-node",
		},
		{
			name:        "wait backup pod running fail",
			ownerBackup: backup,
			nodeName:    "fake-node",
			kubeClientObj: []runtime.Object{
				backupPodNotRunning,
			},
			Timeout: time.Second,
			err:     "error to wait for rediness of pod fake-backup: context deadline exceeded",
		},
		{
			name:        "succeed",
			ownerBackup: backup,
			nodeName:    "fake-node",
			kubeClientObj: []runtime.Object{
				backupPodRunning,
			},
			Timeout: time.Second,
			expectedResult: &ExposeResult{
				ByPod: ExposeByPod{
					HostingPod: backupPodRunning,
					VolumeName: string(backup.UID),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.kubeClientObj...)

			fakeClientBuilder := clientFake.NewClientBuilder()
			fakeClientBuilder = fakeClientBuilder.WithScheme(scheme)

			fakeClient := fakeClientBuilder.WithRuntimeObjects(test.kubeClientObj...).Build()

			exposer := podVolumeExposer{
				kubeClient: fakeKubeClient,
				log:        velerotest.NewLogger(),
			}

			var ownerObject corev1api.ObjectReference
			if test.ownerBackup != nil {
				ownerObject = corev1api.ObjectReference{
					Kind:       test.ownerBackup.Kind,
					Namespace:  test.ownerBackup.Namespace,
					Name:       test.ownerBackup.Name,
					UID:        test.ownerBackup.UID,
					APIVersion: test.ownerBackup.APIVersion,
				}
			}

			result, err := exposer.GetExposed(context.Background(), ownerObject, fakeClient, test.nodeName, test.Timeout)
			if test.err == "" {
				require.NoError(t, err)

				if test.expectedResult == nil {
					assert.Nil(t, result)
				} else {
					require.NoError(t, err)
					assert.Equal(t, test.expectedResult.ByPod.VolumeName, result.ByPod.VolumeName)
					assert.Equal(t, test.expectedResult.ByPod.HostingPod.Name, result.ByPod.HostingPod.Name)
				}
			} else {
				assert.EqualError(t, err, test.err)
			}
		})
	}
}

func TestPodVolumePeekExpose(t *testing.T) {
	backup := &velerov1.Backup{
		TypeMeta: metav1.TypeMeta{
			APIVersion: velerov1.SchemeGroupVersion.String(),
			Kind:       "Backup",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: velerov1.DefaultNamespace,
			Name:      "fake-backup",
			UID:       "fake-uid",
		},
	}

	backupPodUrecoverable := &corev1api.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: backup.Namespace,
			Name:      backup.Name,
		},
		Status: corev1api.PodStatus{
			Phase: corev1api.PodFailed,
		},
	}

	backupPod := &corev1api.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: backup.Namespace,
			Name:      backup.Name,
		},
	}

	scheme := runtime.NewScheme()
	corev1api.AddToScheme(scheme)

	tests := []struct {
		name          string
		kubeClientObj []runtime.Object
		ownerBackup   *velerov1.Backup
		err           string
	}{
		{
			name:        "backup pod is not found",
			ownerBackup: backup,
		},
		{
			name:        "pod is unrecoverable",
			ownerBackup: backup,
			kubeClientObj: []runtime.Object{
				backupPodUrecoverable,
			},
			err: "Pod is in abnormal state [Failed], message []",
		},
		{
			name:        "succeed",
			ownerBackup: backup,
			kubeClientObj: []runtime.Object{
				backupPod,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.kubeClientObj...)

			exposer := podVolumeExposer{
				kubeClient: fakeKubeClient,
				log:        velerotest.NewLogger(),
			}

			var ownerObject corev1api.ObjectReference
			if test.ownerBackup != nil {
				ownerObject = corev1api.ObjectReference{
					Kind:       test.ownerBackup.Kind,
					Namespace:  test.ownerBackup.Namespace,
					Name:       test.ownerBackup.Name,
					UID:        test.ownerBackup.UID,
					APIVersion: test.ownerBackup.APIVersion,
				}
			}

			err := exposer.PeekExposed(context.Background(), ownerObject)
			if test.err == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.err)
			}
		})
	}
}

func TestPodVolumeDiagnoseExpose(t *testing.T) {
	backup := &velerov1.Backup{
		TypeMeta: metav1.TypeMeta{
			APIVersion: velerov1.SchemeGroupVersion.String(),
			Kind:       "Backup",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: velerov1.DefaultNamespace,
			Name:      "fake-backup",
			UID:       "fake-uid",
		},
	}

	backupPodWithoutNodeName := corev1api.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: velerov1.DefaultNamespace,
			Name:      "fake-backup",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: backup.APIVersion,
					Kind:       backup.Kind,
					Name:       backup.Name,
					UID:        backup.UID,
				},
			},
		},
		Status: corev1api.PodStatus{
			Phase: corev1api.PodPending,
			Conditions: []corev1api.PodCondition{
				{
					Type:    corev1api.PodInitialized,
					Status:  corev1api.ConditionTrue,
					Message: "fake-pod-message",
				},
			},
		},
	}

	backupPodWithNodeName := corev1api.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: velerov1.DefaultNamespace,
			Name:      "fake-backup",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: backup.APIVersion,
					Kind:       backup.Kind,
					Name:       backup.Name,
					UID:        backup.UID,
				},
			},
		},
		Spec: corev1api.PodSpec{
			NodeName: "fake-node",
		},
		Status: corev1api.PodStatus{
			Phase: corev1api.PodPending,
			Conditions: []corev1api.PodCondition{
				{
					Type:    corev1api.PodInitialized,
					Status:  corev1api.ConditionTrue,
					Message: "fake-pod-message",
				},
			},
		},
	}

	nodeAgentPod := corev1api.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: velerov1.DefaultNamespace,
			Name:      "node-agent-pod-1",
			Labels:    map[string]string{"role": "node-agent"},
		},
		Spec: corev1api.PodSpec{
			NodeName: "fake-node",
		},
		Status: corev1api.PodStatus{
			Phase: corev1api.PodRunning,
		},
	}

	tests := []struct {
		name              string
		ownerBackup       *velerov1.Backup
		kubeClientObj     []runtime.Object
		snapshotClientObj []runtime.Object
		expected          string
	}{
		{
			name:        "no pod",
			ownerBackup: backup,
			expected: `begin diagnose pod volume exposer
error getting hosting pod fake-backup, err: pods "fake-backup" not found
end diagnose pod volume exposer`,
		},
		{
			name:        "pod without node name, pvc without volume name, vs without status",
			ownerBackup: backup,
			kubeClientObj: []runtime.Object{
				&backupPodWithoutNodeName,
			},
			expected: `begin diagnose pod volume exposer
Pod velero/fake-backup, phase Pending, node name 
Pod condition Initialized, status True, reason , message fake-pod-message
end diagnose pod volume exposer`,
		},
		{
			name:        "pod without node name",
			ownerBackup: backup,
			kubeClientObj: []runtime.Object{
				&backupPodWithoutNodeName,
			},
			expected: `begin diagnose pod volume exposer
Pod velero/fake-backup, phase Pending, node name 
Pod condition Initialized, status True, reason , message fake-pod-message
end diagnose pod volume exposer`,
		},
		{
			name:        "pod with node name, no node agent",
			ownerBackup: backup,
			kubeClientObj: []runtime.Object{
				&backupPodWithNodeName,
			},
			expected: `begin diagnose pod volume exposer
Pod velero/fake-backup, phase Pending, node name fake-node
Pod condition Initialized, status True, reason , message fake-pod-message
node-agent is not running in node fake-node, err: daemonset pod not found in running state in node fake-node
end diagnose pod volume exposer`,
		},
		{
			name:        "pod with node name, node agent is running",
			ownerBackup: backup,
			kubeClientObj: []runtime.Object{
				&backupPodWithNodeName,
				&nodeAgentPod,
			},
			expected: `begin diagnose pod volume exposer
Pod velero/fake-backup, phase Pending, node name fake-node
Pod condition Initialized, status True, reason , message fake-pod-message
end diagnose pod volume exposer`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(tt.kubeClientObj...)
			e := &podVolumeExposer{
				kubeClient: fakeKubeClient,
				log:        velerotest.NewLogger(),
			}
			var ownerObject corev1api.ObjectReference
			if tt.ownerBackup != nil {
				ownerObject = corev1api.ObjectReference{
					Kind:       tt.ownerBackup.Kind,
					Namespace:  tt.ownerBackup.Namespace,
					Name:       tt.ownerBackup.Name,
					UID:        tt.ownerBackup.UID,
					APIVersion: tt.ownerBackup.APIVersion,
				}
			}

			diag := e.DiagnoseExpose(context.Background(), ownerObject)
			assert.Equal(t, tt.expected, diag)
		})
	}
}
