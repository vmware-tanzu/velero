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
	id          string
}

const POD_COUNT, VOLUME_COUNT_PER_POD = 2, 3
const OPT_IN_ANN, OPT_OUT_ANN = "backup.velero.io/backup-volumes", "backup.velero.io/backup-volumes-excludes"
const FILE_NAME = "test-data.txt"

var OptInPVBackupTest func() = TestFunc(&PVBackupFiltering{annotation: OPT_IN_ANN, id: "opt-in"})
var OptOutPVBackupTest func() = TestFunc(&PVBackupFiltering{annotation: OPT_OUT_ANN, id: "opt-out"})

func (p *PVBackupFiltering) Init() error {
	p.Ctx, _ = context.WithTimeout(context.Background(), 60*time.Minute)
	p.VeleroCfg = VeleroCfg
	p.Client = *p.VeleroCfg.ClientToInstallVelero
	p.VeleroCfg.UseVolumeSnapshots = false
	p.VeleroCfg.UseNodeAgent = true
	p.NSBaseName = "ns"
	p.NSIncluded = &[]string{fmt.Sprintf("%s-%s-%d", p.NSBaseName, p.id, 1), fmt.Sprintf("%s-%s-%d", p.NSBaseName, p.id, 2)}

	p.TestMsg = &TestMSG{
		Desc:      "Backup PVs filtering by opt-in/opt-out annotation",
		FailedMSG: "Failed to PVs filtering by opt-in/opt-out annotation",
		Text:      fmt.Sprintf("Should backup PVs in namespace %s according to annotation %s", *p.NSIncluded, p.annotation),
	}
	return nil
}

func (p *PVBackupFiltering) StartRun() error {
	err := InstallStorageClass(context.Background(), fmt.Sprintf("testdata/storage-class/%s.yaml", VeleroCfg.CloudProvider))
	if err != nil {
		return err
	}
	p.BackupName = p.BackupName + "backup-" + p.id + "-" + UUIDgen.String()
	p.RestoreName = p.RestoreName + "restore-" + p.id + "-" + UUIDgen.String()
	p.BackupArgs = []string{
		"create", "--namespace", VeleroCfg.VeleroNamespace, "backup", p.BackupName,
		"--include-namespaces", strings.Join(*p.NSIncluded, ","),
		"--snapshot-volumes=false", "--wait",
	}
	// "--default-volumes-to-fs-backup" is an overall switch, if it's set, then opt-in
	//   annotation will be ignored, so it's only set for opt-out test
	if p.annotation == OPT_OUT_ANN {
		p.BackupArgs = append(p.BackupArgs, "--default-volumes-to-fs-backup")

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
		By(fmt.Sprintf("Deploy a few pods with several PVs in namespace %s", ns), func() {
			var volumesToAnnotation string
			//Make sure PVC name is unique from other tests to avoid PVC creation error
			for i := 0; i <= POD_COUNT-1; i++ {
				var volumeToAnnotationList []string
				var volumes []string
				for j := 0; j <= VOLUME_COUNT_PER_POD-1; j++ {
					volume := fmt.Sprintf("volume-%s-%d-%d", p.id, i, j)
					volumes = append(volumes, volume)
					//Volumes cherry-pick policy for opt-in/out annotation to apply
					if j%2 == 0 {
						volumeToAnnotationList = append(volumeToAnnotationList, volume)
					}
				}
				p.volumesList = append(p.volumesList, volumes)
				volumesToAnnotation = strings.Join(volumeToAnnotationList, ",")
				podName := fmt.Sprintf("pod-%d", i)
				pods = append(pods, podName)
				By(fmt.Sprintf("Create pod %s in namespace %s", podName, ns), func() {
					pod, err := CreatePod(p.Client, ns, podName, "e2e-storage-class", "", volumes, nil, nil)
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
	By(fmt.Sprintf("Populate all pods %s with file %s", p.podsList, FILE_NAME), func() {
		for index, ns := range *p.NSIncluded {
			By(fmt.Sprintf("Creating file in all pods to start %d in namespace %s", index, ns), func() {
				WaitForPods(p.Ctx, p.Client, ns, p.podsList[index])
				for i, pod := range p.podsList[index] {
					for j := range p.volumesList[i] {
						Expect(CreateFileToPod(p.Ctx, ns, pod, p.volumesList[i][j],
							FILE_NAME, fileContent(ns, pod, p.volumesList[i][j]))).To(Succeed())
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
					// Same with volumes cherry pick policy to verify backup result
					if j%2 == 0 {
						if p.annotation == OPT_IN_ANN {
							By(fmt.Sprintf("File should exists in PV %s of pod %s under namespace %s\n", p.volumesList[i][j], p.podsList[k][i], ns), func() {
								Expect(fileExist(p.Ctx, ns, p.podsList[k][i], p.volumesList[i][j])).To(Succeed(), "File not exist as expect")
							})
						} else {
							By(fmt.Sprintf("File should not exist in PV %s of pod %s under namespace %s\n", p.volumesList[i][j], p.podsList[k][i], ns), func() {
								Expect(fileNotExist(p.Ctx, ns, p.podsList[k][i], p.volumesList[i][j])).To(Succeed(), "File exists, not as expect")
							})
						}
					} else {
						if p.annotation == OPT_OUT_ANN {
							By(fmt.Sprintf("File should exists in PV %s of pod %s under namespace %s\n", p.volumesList[i][j], p.podsList[k][i], ns), func() {
								Expect(fileExist(p.Ctx, ns, p.podsList[k][i], p.volumesList[i][j])).To(Succeed(), "File not exist as expect")
							})
						} else {
							By(fmt.Sprintf("File should not exist in PV %s of pod %s under namespace %s\n", p.volumesList[i][j], p.podsList[k][i], ns), func() {
								Expect(fileNotExist(p.Ctx, ns, p.podsList[k][i], p.volumesList[i][j])).To(Succeed(), "File exists, not as expect")
							})
						}
					}
				}
			}
		})
	}

	return nil
}
func fileContent(namespace, podName, volume string) string {
	return fmt.Sprintf("ns-%s pod-%s volume-%s", namespace, podName, volume)
}

func fileExist(ctx context.Context, namespace, podName, volume string) error {
	c, err := ReadFileFromPodVolume(ctx, namespace, podName, volume, FILE_NAME)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Fail to read file %s from volume %s of pod %s in %s ",
			FILE_NAME, volume, podName, namespace))
	}
	c = strings.Replace(c, "\n", "", -1)
	origin_content := strings.Replace(fileContent(namespace, podName, volume), "\n", "", -1)
	if c == origin_content {
		return nil
	} else {
		return errors.New(fmt.Sprintf("UNEXPECTED: File %s does not exist in volume %s of pod %s in namespace %s.",
			FILE_NAME, volume, podName, namespace))
	}
}
func fileNotExist(ctx context.Context, namespace, podName, volume string) error {
	_, err := ReadFileFromPodVolume(ctx, namespace, podName, volume, FILE_NAME)
	if err != nil {
		return nil
	} else {
		return errors.New(fmt.Sprintf("UNEXPECTED: File %s exist in volume %s of pod %s in namespace %s.",
			FILE_NAME, volume, podName, namespace))
	}
}
