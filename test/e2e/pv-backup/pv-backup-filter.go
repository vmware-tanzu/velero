package basic

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	. "github.com/vmware-tanzu/velero/test/e2e"
	. "github.com/vmware-tanzu/velero/test/e2e/test"
	. "github.com/vmware-tanzu/velero/test/e2e/util/k8s"
)

type PVBackupFiltering struct {
	TestCase
	annotation  string
	podsList    [][]string
	volumesList [][]string
}

const POD_COUNT, VOLUME_COUNT_PER_POD = 2, 3
const OPT_IN_ANN, OPT_OUT_ANN = "backup.velero.io/backup-volumes", "backup.velero.io/backup-volumes-excludes"
const FILE_NAME = "test-data.txt"

var OptInPVBackupTest func() = TestFunc(&PVBackupFiltering{annotation: OPT_IN_ANN})
var OptOutPVBackupTest func() = TestFunc(&PVBackupFiltering{annotation: OPT_OUT_ANN})

func (p *PVBackupFiltering) Init() error {
	p.Client = TestClientInstance
	p.NSBaseName = "ns"
	p.NSIncluded = &[]string{fmt.Sprintf("%s-%d", p.NSBaseName, 1), fmt.Sprintf("%s-%d", p.NSBaseName, 2)}

	p.TestMsg = &TestMSG{
		Desc:      "Backup PV filter by annotation",
		FailedMSG: "Failed to backup PV filter by annotation",
		Text:      fmt.Sprintf("should backup namespaces %s", *p.NSIncluded),
	}
	return nil
}

func (p *PVBackupFiltering) StartRun() error {
	p.BackupName = p.BackupName + "backup-opt-in-" + UUIDgen.String()
	p.RestoreName = p.RestoreName + "restore-opt-in-" + UUIDgen.String()
	p.BackupArgs = []string{
		"create", "--namespace", VeleroCfg.VeleroNamespace, "backup", p.BackupName,
		"--include-namespaces", strings.Join(*p.NSIncluded, ","),
		"--default-volumes-to-restic", "--snapshot-volumes=false", "--wait",
	}
	p.RestoreArgs = []string{
		"create", "--namespace", VeleroCfg.VeleroNamespace, "restore", p.RestoreName,
		"--from-backup", p.BackupName, "--wait",
	}
	return nil
}
func (p *PVBackupFiltering) CreateResources() error {
	p.Ctx, _ = context.WithTimeout(context.Background(), 60*time.Minute)
	for _, ns := range *p.NSIncluded {
		By(fmt.Sprintf("Create namespaces %s for workload\n", ns), func() {
			Expect(CreateNamespace(p.Ctx, p.Client, ns)).To(Succeed(), fmt.Sprintf("Failed to create namespace %s", ns))
		})
		var pods []string
		By(fmt.Sprintf("Deploy more pods with several PVs in namespace %s", ns), func() {
			var volumesToAnnotation string
			for i := 0; i <= POD_COUNT-1; i++ {
				var volumeToAnnotationList []string
				var volumes []string
				for j := 0; j <= VOLUME_COUNT_PER_POD-1; j++ {
					volume := fmt.Sprintf("volume-%d-%d", i, j)
					volumes = append(volumes, volume)
					//Policy for apply annotation
					if j%2 == 0 {
						volumeToAnnotationList = append(volumeToAnnotationList, volume)
					}
				}
				p.volumesList = append(p.volumesList, volumes)
				volumesToAnnotation = strings.Join(volumeToAnnotationList, ",")
				podName := fmt.Sprintf("pod-%d", i)
				pods = append(pods, podName)
				By(fmt.Sprintf("Create pod %s in namespace %s", podName, ns), func() {
					pod, err := CreatePodWithPVC(p.Client, ns, podName, "kibishii-storage-class", volumes)
					Expect(err).To(Succeed())
					ann := map[string]string{
						p.annotation: volumesToAnnotation,
					}
					By(fmt.Sprintf("Add annotation to pod %s of namespace %s", pod.Name, ns), func() {
						_, err := AddAnnotationToPod(p.Ctx, p.Client, ns, pod.Name, ann)
						Expect(err).To(Succeed())
					})
				})
			}
		})
		p.podsList = append(p.podsList, pods)
	}
	By(fmt.Sprintf("Waiting for all pods to start %s\n", p.podsList), func() {
		for index, ns := range *p.NSIncluded {
			By(fmt.Sprintf("Waiting for all pods to start %d in namespace %s", index, ns), func() {
				WaitForPods(p.Ctx, p.Client, ns, p.podsList[index])
			})
		}
	})
	By(fmt.Sprintf("Polulate all pods %s with file %s", p.podsList, FILE_NAME), func() {
		for index, ns := range *p.NSIncluded {
			By(fmt.Sprintf("Creating file in all pods to start %d in namespace %s", index, ns), func() {
				WaitForPods(p.Ctx, p.Client, ns, p.podsList[index])
				for i, pod := range p.podsList[index] {
					for j, _ := range p.volumesList[i] {
						Expect(CreateFileToPod(p.Ctx, ns, pod, p.volumesList[i][j],
							FILE_NAME, FileContent(ns, pod, p.volumesList[i][j]))).To(Succeed())
					}
				}
			})
		}
	})
	return nil
}

func (p *PVBackupFiltering) Verify() error {
	p.Ctx, _ = context.WithTimeout(context.Background(), 60*time.Minute)
	By(fmt.Sprintf("Waiting for all pods to start %s", p.podsList), func() {
		for index, ns := range *p.NSIncluded {
			By(fmt.Sprintf("Waiting for all pods to start %d in namespace %s", index, ns), func() {
				WaitForPods(p.Ctx, p.Client, ns, p.podsList[index])
			})
		}
	})

	for k, ns := range *p.NSIncluded {
		By("Verify PV backed up according to annotation", func() {
			for i := 0; i <= POD_COUNT-1; i++ {
				for j := 0; j <= VOLUME_COUNT_PER_POD-1; j++ {
					if j%2 == 0 {
						if p.annotation == OPT_IN_ANN {
							Expect(FileExist(p.Ctx, ns, p.podsList[k][i], p.volumesList[i][j])).To(Succeed())
						} else {
							Expect(FileNotExist(p.Ctx, ns, p.podsList[k][i], p.volumesList[i][j])).To(Succeed())
						}
					} else {
						if p.annotation == OPT_OUT_ANN {
							Expect(FileExist(p.Ctx, ns, p.podsList[k][i], p.volumesList[i][j])).To(Succeed())
						} else {
							Expect(FileNotExist(p.Ctx, ns, p.podsList[k][i], p.volumesList[i][j])).To(Succeed())
						}
					}
				}
			}
		})
	}

	return nil
}
func FileContent(namespace, podName, volume string) string {
	return fmt.Sprintf("ns-%s pod-%s volume-%s", namespace, podName, volume)
}

func FileExist(ctx context.Context, namespace, podName, volume string) error {
	c, err := ReadFileFromPodVolume(ctx, namespace, podName, volume, FILE_NAME)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Fail to read file %s from volume %s of pod %s in %s ",
			FILE_NAME, volume, podName, namespace))
	}
	c = strings.Replace(c, "\n", "", -1)
	origin_content := strings.Replace(FileContent(namespace, podName, volume), "\n", "", -1)
	if c == origin_content {
		return nil
	} else {
		return errors.New(fmt.Sprintf("UNEXPECTED: File %s does not exsit in volume %s of pod %s in namespace %s.",
			FILE_NAME, volume, podName, namespace))
	}
}
func FileNotExist(ctx context.Context, namespace, podName, volume string) error {
	_, err := ReadFileFromPodVolume(ctx, namespace, podName, volume, FILE_NAME)
	if err != nil {
		return nil
	} else {
		return errors.New(fmt.Sprintf("UNEXPECTED: File %s exsit in volume %s of pod %s in namespace %s.",
			FILE_NAME, volume, podName, namespace))
	}
}
