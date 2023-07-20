/*
Copyright 2018 the Velero contributors.

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

package podvolume

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clientTesting "k8s.io/client-go/testing"

	"github.com/vmware-tanzu/velero/internal/resourcepolicies"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned"
	velerofake "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/fake"
	"github.com/vmware-tanzu/velero/pkg/repository"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestIsHostPathVolume(t *testing.T) {
	// hostPath pod volume
	vol := &corev1api.Volume{
		VolumeSource: corev1api.VolumeSource{
			HostPath: &corev1api.HostPathVolumeSource{},
		},
	}
	isHostPath, err := isHostPathVolume(vol, nil, nil)
	assert.Nil(t, err)
	assert.True(t, isHostPath)

	// non-hostPath pod volume
	vol = &corev1api.Volume{
		VolumeSource: corev1api.VolumeSource{
			EmptyDir: &corev1api.EmptyDirVolumeSource{},
		},
	}
	isHostPath, err = isHostPathVolume(vol, nil, nil)
	assert.Nil(t, err)
	assert.False(t, isHostPath)

	// PVC that doesn't have a PV
	vol = &corev1api.Volume{
		VolumeSource: corev1api.VolumeSource{
			PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
				ClaimName: "pvc-1",
			},
		},
	}
	pvc := &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns-1",
			Name:      "pvc-1",
		},
	}
	isHostPath, err = isHostPathVolume(vol, pvc, nil)
	assert.Nil(t, err)
	assert.False(t, isHostPath)

	// PVC that claims a non-hostPath PV
	vol = &corev1api.Volume{
		VolumeSource: corev1api.VolumeSource{
			PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
				ClaimName: "pvc-1",
			},
		},
	}
	pvc = &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns-1",
			Name:      "pvc-1",
		},
		Spec: corev1api.PersistentVolumeClaimSpec{
			VolumeName: "pv-1",
		},
	}
	pvGetter := &fakePVGetter{
		pv: &corev1api.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pv-1",
			},
			Spec: corev1api.PersistentVolumeSpec{},
		},
	}
	isHostPath, err = isHostPathVolume(vol, pvc, pvGetter)
	assert.Nil(t, err)
	assert.False(t, isHostPath)

	// PVC that claims a hostPath PV
	vol = &corev1api.Volume{
		VolumeSource: corev1api.VolumeSource{
			PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
				ClaimName: "pvc-1",
			},
		},
	}
	pvc = &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns-1",
			Name:      "pvc-1",
		},
		Spec: corev1api.PersistentVolumeClaimSpec{
			VolumeName: "pv-1",
		},
	}
	pvGetter = &fakePVGetter{
		pv: &corev1api.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pv-1",
			},
			Spec: corev1api.PersistentVolumeSpec{
				PersistentVolumeSource: corev1api.PersistentVolumeSource{
					HostPath: &corev1api.HostPathVolumeSource{},
				},
			},
		},
	}
	isHostPath, err = isHostPathVolume(vol, pvc, pvGetter)
	assert.Nil(t, err)
	assert.True(t, isHostPath)
}

type fakePVGetter struct {
	pv *corev1api.PersistentVolume
}

func (g *fakePVGetter) Get(ctx context.Context, name string, opts metav1.GetOptions) (*corev1api.PersistentVolume, error) {
	if g.pv != nil {
		return g.pv, nil
	}

	return nil, errors.New("item not found")
}

func Test_backupper_BackupPodVolumes_log_test(t *testing.T) {
	type args struct {
		backup          *velerov1api.Backup
		pod             *corev1api.Pod
		volumesToBackup []string
		resPolicies     *resourcepolicies.Policies
	}
	tests := []struct {
		name    string
		args    args
		wantLog string
	}{
		{
			name: "backup pod volumes should log volume names",
			args: args{
				backup: &velerov1api.Backup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "backup-1",
						Namespace: "ns-1",
					},
				},
				pod: &corev1api.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-1",
						Namespace: "ns-1",
					},
					Spec: corev1api.PodSpec{
						Volumes: []corev1api.Volume{
							{
								Name: "vol-1",
							},
							{
								Name: "vol-2",
							},
						},
					},
				},
				volumesToBackup: []string{"vol-1", "vol-2"},
				resPolicies:     nil,
			},
			wantLog: "pod ns-1/pod-1 has volumes to backup: [vol-1 vol-2]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &backupper{
				ctx: context.Background(),
			}
			logOutput := bytes.Buffer{}
			var log = logrus.New()
			log.SetOutput(&logOutput)
			b.BackupPodVolumes(tt.args.backup, tt.args.pod, tt.args.volumesToBackup, tt.args.resPolicies, log)
			fmt.Println(logOutput.String())
			assert.Contains(t, logOutput.String(), tt.wantLog)

		})
	}
}

type reactor struct {
	verb        string
	resource    string
	reactorFunc clientTesting.ReactionFunc
}

func createBackupRepoObj() *velerov1api.BackupRepository {
	bkRepoObj := repository.NewBackupRepository(velerov1api.DefaultNamespace, repository.BackupRepositoryKey{
		VolumeNamespace: "fake-ns",
		BackupLocation:  "fake-bsl",
		RepositoryType:  "kopia",
	})

	bkRepoObj.Status.Phase = velerov1api.BackupRepositoryPhaseReady

	return bkRepoObj
}

func createPodObj(running bool, withVolume bool, withVolumeMounted bool, volumeNum int) *corev1api.Pod {
	podObj := builder.ForPod("fake-ns", "fake-pod").Result()
	podObj.Spec.NodeName = "fake-node-name"
	if running {
		podObj.Status.Phase = corev1api.PodRunning
	}

	if withVolume {
		for i := 0; i < volumeNum; i++ {
			podObj.Spec.Volumes = append(podObj.Spec.Volumes, corev1api.Volume{
				Name: fmt.Sprintf("fake-volume-%d", i+1),
				VolumeSource: corev1api.VolumeSource{
					PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
						ClaimName: fmt.Sprintf("fake-pvc-%d", i+1),
					},
				},
			})
		}

		if withVolumeMounted {
			volumeMount := []corev1api.VolumeMount{}
			for i := 0; i < volumeNum; i++ {
				volumeMount = append(volumeMount, corev1api.VolumeMount{
					Name: fmt.Sprintf("fake-volume-%d", i+1),
				})
			}
			podObj.Spec.Containers = []corev1api.Container{
				{
					Name:         "fake-container",
					VolumeMounts: volumeMount,
				},
			}
		}
	}

	return podObj
}

func createNodeAgentPodObj(running bool) *corev1api.Pod {
	podObj := builder.ForPod(velerov1api.DefaultNamespace, "fake-node-agent").Result()
	podObj.Labels = map[string]string{"name": "node-agent"}

	if running {
		podObj.Status.Phase = corev1api.PodRunning
		podObj.Spec.NodeName = "fake-node-name"
	}

	return podObj
}

func createPVObj(index int, withHostPath bool) *corev1api.PersistentVolume {
	pvObj := builder.ForPersistentVolume(fmt.Sprintf("fake-pv-%d", index)).Result()
	if withHostPath {
		pvObj.Spec.HostPath = &corev1api.HostPathVolumeSource{Path: "fake-host-path"}
	}

	return pvObj
}

func createPVCObj(index int) *corev1api.PersistentVolumeClaim {
	pvcObj := builder.ForPersistentVolumeClaim("fake-ns", fmt.Sprintf("fake-pvc-%d", index)).VolumeName(fmt.Sprintf("fake-pv-%d", index)).Result()
	return pvcObj
}

func createPVBObj(fail bool, withSnapshot bool, index int, uploaderType string) *velerov1api.PodVolumeBackup {
	pvbObj := builder.ForPodVolumeBackup(velerov1api.DefaultNamespace, fmt.Sprintf("fake-pvb-%d", index)).
		PodName("fake-pod").PodNamespace("fake-ns").Volume(fmt.Sprintf("fake-volume-%d", index)).Result()
	if fail {
		pvbObj.Status.Phase = velerov1api.PodVolumeBackupPhaseFailed
		pvbObj.Status.Message = "fake-message"
	} else {
		pvbObj.Status.Phase = velerov1api.PodVolumeBackupPhaseCompleted
	}

	if withSnapshot {
		pvbObj.Status.SnapshotID = fmt.Sprintf("fake-snapshot-id-%d", index)
	}

	pvbObj.Spec.UploaderType = uploaderType

	return pvbObj
}

func TestBackupPodVolumes(t *testing.T) {
	scheme := runtime.NewScheme()
	velerov1api.AddToScheme(scheme)

	ctxWithCancel, cancel := context.WithCancel(context.Background())
	defer cancel()

	failedPVB := createPVBObj(true, false, 1, "")
	completedPVB := createPVBObj(false, false, 1, "")

	tests := []struct {
		name            string
		ctx             context.Context
		bsl             string
		uploaderType    string
		volumes         []string
		sourcePod       *corev1api.Pod
		kubeClientObj   []runtime.Object
		ctlClientObj    []runtime.Object
		veleroClientObj []runtime.Object
		veleroReactors  []reactor
		runtimeScheme   *runtime.Scheme
		retPVBs         []*velerov1api.PodVolumeBackup
		pvbs            []*velerov1api.PodVolumeBackup
		errs            []string
	}{
		{
			name: "empty volume list",
		},
		{
			name: "pod is not running",
			volumes: []string{
				"fake-volume-1",
				"fake-volume-2",
			},
			kubeClientObj: []runtime.Object{
				createNodeAgentPodObj(true),
			},
			ctlClientObj: []runtime.Object{
				createBackupRepoObj(),
			},
			runtimeScheme: scheme,
			sourcePod:     createPodObj(false, false, false, 2),
			uploaderType:  "kopia",
			bsl:           "fake-bsl",
		},
		{
			name: "node-agent pod is not running in node",
			volumes: []string{
				"fake-volume-1",
				"fake-volume-2",
			},
			sourcePod: createPodObj(true, false, false, 2),
			errs: []string{
				"daemonset pod not found in running state in node fake-node-name",
			},
		},
		{
			name: "wrong repository type",
			volumes: []string{
				"fake-volume-1",
				"fake-volume-2",
			},
			sourcePod: createPodObj(true, false, false, 2),
			kubeClientObj: []runtime.Object{
				createNodeAgentPodObj(true),
			},
			uploaderType: "fake-uploader-type",
			errs: []string{
				"empty repository type, uploader fake-uploader-type",
			},
		},
		{
			name: "ensure repo fail",
			volumes: []string{
				"fake-volume-1",
				"fake-volume-2",
			},
			sourcePod: createPodObj(true, false, false, 2),
			kubeClientObj: []runtime.Object{
				createNodeAgentPodObj(true),
			},
			uploaderType: "kopia",
			errs: []string{
				"wrong parameters, namespace \"fake-ns\", backup storage location \"\", repository type \"kopia\"",
			},
		},
		{
			name: "volume not found in pod",
			volumes: []string{
				"fake-volume-1",
				"fake-volume-2",
			},
			sourcePod: createPodObj(true, false, false, 2),
			kubeClientObj: []runtime.Object{
				createNodeAgentPodObj(true),
			},
			ctlClientObj: []runtime.Object{
				createBackupRepoObj(),
			},
			runtimeScheme: scheme,
			uploaderType:  "kopia",
			bsl:           "fake-bsl",
		},
		{
			name: "PVC not found",
			volumes: []string{
				"fake-volume-1",
				"fake-volume-2",
			},
			sourcePod: createPodObj(true, true, false, 2),
			kubeClientObj: []runtime.Object{
				createNodeAgentPodObj(true),
			},
			ctlClientObj: []runtime.Object{
				createBackupRepoObj(),
			},
			runtimeScheme: scheme,
			uploaderType:  "kopia",
			bsl:           "fake-bsl",
			errs: []string{
				"error getting persistent volume claim for volume: persistentvolumeclaims \"fake-pvc-1\" not found",
				"error getting persistent volume claim for volume: persistentvolumeclaims \"fake-pvc-2\" not found",
			},
		},
		{
			name: "check host path fail",
			volumes: []string{
				"fake-volume-1",
				"fake-volume-2",
			},
			sourcePod: createPodObj(true, true, false, 2),
			kubeClientObj: []runtime.Object{
				createNodeAgentPodObj(true),
				createPVCObj(1),
				createPVCObj(2),
			},
			ctlClientObj: []runtime.Object{
				createBackupRepoObj(),
			},
			runtimeScheme: scheme,
			uploaderType:  "kopia",
			bsl:           "fake-bsl",
			errs: []string{
				"error checking if volume is a hostPath volume: persistentvolumes \"fake-pv-1\" not found",
				"error checking if volume is a hostPath volume: persistentvolumes \"fake-pv-2\" not found",
			},
		},
		{
			name: "host path volume should be skipped",
			volumes: []string{
				"fake-volume-1",
				"fake-volume-2",
			},
			sourcePod: createPodObj(true, true, false, 2),
			kubeClientObj: []runtime.Object{
				createNodeAgentPodObj(true),
				createPVCObj(1),
				createPVCObj(2),
				createPVObj(1, true),
				createPVObj(2, true),
			},
			ctlClientObj: []runtime.Object{
				createBackupRepoObj(),
			},
			runtimeScheme: scheme,
			uploaderType:  "kopia",
			bsl:           "fake-bsl",
		},
		{
			name: "volume not mounted by pod should be skipped",
			volumes: []string{
				"fake-volume-1",
				"fake-volume-2",
			},
			sourcePod: createPodObj(true, true, false, 2),
			kubeClientObj: []runtime.Object{
				createNodeAgentPodObj(true),
				createPVCObj(1),
				createPVCObj(2),
				createPVObj(1, false),
				createPVObj(2, false),
			},
			ctlClientObj: []runtime.Object{
				createBackupRepoObj(),
			},
			runtimeScheme: scheme,
			uploaderType:  "kopia",
			bsl:           "fake-bsl",
		},
		{
			name: "create PVB fail",
			volumes: []string{
				"fake-volume-1",
				"fake-volume-2",
			},
			sourcePod: createPodObj(true, true, true, 2),
			kubeClientObj: []runtime.Object{
				createNodeAgentPodObj(true),
				createPVCObj(1),
				createPVCObj(2),
				createPVObj(1, false),
				createPVObj(2, false),
			},
			ctlClientObj: []runtime.Object{
				createBackupRepoObj(),
			},
			veleroReactors: []reactor{
				{
					verb:     "create",
					resource: "podvolumebackups",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-create-error")
					},
				},
			},
			runtimeScheme: scheme,
			uploaderType:  "kopia",
			bsl:           "fake-bsl",
			errs: []string{
				"fake-create-error",
				"fake-create-error",
			},
		},
		{
			name: "context cancelled",
			ctx:  ctxWithCancel,
			volumes: []string{
				"fake-volume-1",
			},
			sourcePod: createPodObj(true, true, true, 1),
			kubeClientObj: []runtime.Object{
				createNodeAgentPodObj(true),
				createPVCObj(1),
				createPVObj(1, false),
			},
			ctlClientObj: []runtime.Object{
				createBackupRepoObj(),
			},
			runtimeScheme: scheme,
			uploaderType:  "kopia",
			bsl:           "fake-bsl",
			errs: []string{
				"timed out waiting for all PodVolumeBackups to complete",
			},
		},
		{
			name: "return failed pvbs",
			volumes: []string{
				"fake-volume-1",
			},
			sourcePod: createPodObj(true, true, true, 1),
			kubeClientObj: []runtime.Object{
				createNodeAgentPodObj(true),
				createPVCObj(1),
				createPVObj(1, false),
			},
			ctlClientObj: []runtime.Object{
				createBackupRepoObj(),
			},
			runtimeScheme: scheme,
			uploaderType:  "kopia",
			bsl:           "fake-bsl",
			retPVBs: []*velerov1api.PodVolumeBackup{
				failedPVB,
			},
			pvbs: []*velerov1api.PodVolumeBackup{
				failedPVB,
			},
			errs: []string{
				"pod volume backup failed: fake-message",
			},
		},
		{
			name: "return completed pvbs",
			volumes: []string{
				"fake-volume-1",
			},
			sourcePod: createPodObj(true, true, true, 1),
			kubeClientObj: []runtime.Object{
				createNodeAgentPodObj(true),
				createPVCObj(1),
				createPVObj(1, false),
			},
			ctlClientObj: []runtime.Object{
				createBackupRepoObj(),
			},
			runtimeScheme: scheme,
			uploaderType:  "kopia",
			bsl:           "fake-bsl",
			retPVBs: []*velerov1api.PodVolumeBackup{
				completedPVB,
			},
			pvbs: []*velerov1api.PodVolumeBackup{
				completedPVB,
			},
		},
	}
	// TODO add more verification around PVCBackupSummary returned by "BackupPodVolumes"
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			if test.ctx != nil {
				ctx = test.ctx
			}

			fakeClientBuilder := ctrlfake.NewClientBuilder()
			if test.runtimeScheme != nil {
				fakeClientBuilder = fakeClientBuilder.WithScheme(test.runtimeScheme)
			}

			fakeCtlClient := fakeClientBuilder.WithRuntimeObjects(test.ctlClientObj...).Build()

			fakeKubeClient := kubefake.NewSimpleClientset(test.kubeClientObj...)
			var kubeClient kubernetes.Interface = fakeKubeClient

			fakeVeleroClient := velerofake.NewSimpleClientset(test.veleroClientObj...)
			for _, reactor := range test.veleroReactors {
				fakeVeleroClient.Fake.PrependReactor(reactor.verb, reactor.resource, reactor.reactorFunc)
			}
			var veleroClient versioned.Interface = fakeVeleroClient

			ensurer := repository.NewEnsurer(fakeCtlClient, velerotest.NewLogger(), time.Millisecond)

			backupObj := builder.ForBackup(velerov1api.DefaultNamespace, "fake-backup").Result()
			backupObj.Spec.StorageLocation = test.bsl

			factory := NewBackupperFactory(repository.NewRepoLocker(), ensurer, veleroClient, kubeClient.CoreV1(), kubeClient.CoreV1(), kubeClient.CoreV1(), velerotest.NewLogger())
			bp, err := factory.NewBackupper(ctx, backupObj, test.uploaderType)

			require.NoError(t, err)

			go func() {
				if test.ctx != nil {
					time.Sleep(time.Second)
					cancel()
				} else if test.retPVBs != nil {
					time.Sleep(time.Second)
					for _, pvb := range test.retPVBs {
						bp.(*backupper).results[resultsKey(test.sourcePod.Namespace, test.sourcePod.Name)] <- pvb
					}

				}
			}()

			pvbs, _, errs := bp.BackupPodVolumes(backupObj, test.sourcePod, test.volumes, nil, velerotest.NewLogger())

			if errs == nil {
				assert.Nil(t, test.errs)
			} else {
				for i := 0; i < len(errs); i++ {
					assert.EqualError(t, errs[i], test.errs[i])
				}
			}

			assert.Equal(t, test.pvbs, pvbs)
		})
	}
}

func TestPVCBackupSummary(t *testing.T) {
	pbs := NewPVCBackupSummary()
	pbs.pvcMap["vol-1"] = builder.ForPersistentVolumeClaim("ns-1", "pvc-1").VolumeName("pv-1").Result()
	pbs.pvcMap["vol-2"] = builder.ForPersistentVolumeClaim("ns-2", "pvc-2").VolumeName("pv-2").Result()

	// it won't be added if the volme is not in the pvc map.
	pbs.addSkipped("vol-3", "whatever reason")
	assert.Equal(t, 0, len(pbs.Skipped))
	pbs.addBackedup("vol-3")
	assert.Equal(t, 0, len(pbs.Backedup))

	// only can be added as skipped when it's not in backedup set
	pbs.addBackedup("vol-1")
	assert.Equal(t, 1, len(pbs.Backedup))
	assert.Equal(t, "pvc-1", pbs.Backedup["vol-1"].Name)
	pbs.addSkipped("vol-1", "whatever reason")
	assert.Equal(t, 0, len(pbs.Skipped))
	pbs.addSkipped("vol-2", "vol-2 has to be skipped")
	assert.Equal(t, 1, len(pbs.Skipped))
	assert.Equal(t, "pvc-2", pbs.Skipped["vol-2"].PVC.Name)

	// adding a vol as backedup removes it from skipped set
	pbs.addBackedup("vol-2")
	assert.Equal(t, 0, len(pbs.Skipped))
	assert.Equal(t, 2, len(pbs.Backedup))
}
