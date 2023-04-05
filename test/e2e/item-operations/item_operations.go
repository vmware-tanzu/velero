package itemoperations

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/vmware-tanzu/velero/test/e2e"
	. "github.com/vmware-tanzu/velero/test/e2e/test"
	. "github.com/vmware-tanzu/velero/test/util/k8s"
)

type ItemOperations struct {
	TestCase
	biaOpDuration  time.Duration
	biaOpExtraItem bool
	podsList    [][]string
	id          string
}

const (
	AsyncBIADurationAnnotation         = "velero.io/example-bia-operation-duration"
	AsyncBIAAdditionalUpdateAnnotation = "velero.io/example-bia-additional-update"
	AsyncBIAExampleSecretAnnotation    = "velero.io/example-bia-secret"
	AsyncBIAPodCount = 2
)


var BIAOperationBackupTest func() = TestFunc(&ItemOperations{biaOpDuration: 2 * time.Minute, id: "2-minutes"})
var BIAOperationExtrasBackupTest func() = TestFunc(&ItemOperations{biaOpDuration: 2 * time.Minute, biaOpExtraItem: true, id: "2-minutes-extra"})

func (o *ItemOperations) Init() error {
	o.TestCase.Init()
	o.Ctx, _ = context.WithTimeout(context.Background(), 60*time.Minute)
	o.VeleroCfg = VeleroCfg
	o.Client = *o.VeleroCfg.ClientToInstallVelero
	o.VeleroCfg.UseVolumeSnapshots = false
	o.VeleroCfg.UseNodeAgent = false
	o.CaseBaseName = "async-" + o.UUIDgen
	o.NamespacesTotal = 2
	o.NSIncluded = &[]string{}
	for nsNum := 0; nsNum < o.NamespacesTotal; nsNum++ {
		createNSName := fmt.Sprintf("%s-%s-%00000d", o.CaseBaseName, o.id, nsNum)
		*o.NSIncluded = append(*o.NSIncluded, createNSName)
	}

	// fixme --- remove
	//o.NSIncluded = &[]string{fmt.Sprintf("%s-%s-%d", o.NSBaseName, o.id, 1), fmt.Sprintf("%s-%s-%d", o.NSBaseName, o.id, 2)}

	o.TestMsg = &TestMSG{
		Desc:      "BIA Operation",
		FailedMSG: "Failed BIA Operation",
		Text:      fmt.Sprintf("Should backup Pod with asynchronous operation in namespace %s", *o.NSIncluded),
	}
	o.BackupName = "backup-" + o.CaseBaseName
	o.RestoreName = "restore-" + o.CaseBaseName
	o.BackupArgs = []string{
		"create", "--namespace", VeleroCfg.VeleroNamespace, "backup", o.BackupName,
		"--include-namespaces", strings.Join(*o.NSIncluded, ","),
		"--snapshot-volumes=false", "--wait",
	}
	o.RestoreArgs = []string{
		"create", "--namespace", VeleroCfg.VeleroNamespace, "restore", o.RestoreName,
		"--from-backup", o.BackupName, "--wait",
	}
	return nil
}
func (o *ItemOperations) CreateResources() error {
	o.Ctx, _ = context.WithTimeout(context.Background(), 60*time.Minute)
	for _, ns := range *o.NSIncluded {
		By(fmt.Sprintf("Create namespaces %s for workload\n", ns), func() {
			Expect(CreateNamespace(o.Ctx, o.Client, ns)).To(Succeed(), fmt.Sprintf("Failed to create namespace %s", ns))
		})
		var pods []string
		By(fmt.Sprintf("Deploy a few pods in namespace %s", ns), func() {
			for i := 0; i <= AsyncBIAPodCount-1; i++ {
				podName := fmt.Sprintf("pod-%d", i)
				pods = append(pods, podName)
				By(fmt.Sprintf("Create pod %s in namespace %s", podName, ns), func() {
					pod, err := CreatePod(o.Client, ns, podName, "", "", []string{}, nil, nil)
					Expect(err).To(Succeed())
					ann := map[string]string{
						AsyncBIADurationAnnotation: o.biaOpDuration.String(),
					}
					if o.biaOpExtraItem {
						ann[AsyncBIAAdditionalUpdateAnnotation] = "true"
					}
					By(fmt.Sprintf("Add annotation to pod %s of namespace %s", pod.Name, ns), func() {
						_, err := AddAnnotationToPod(o.Ctx, o.Client, ns, pod.Name, ann)
						Expect(err).To(Succeed())
					})
				})
			}
		})
		o.podsList = append(o.podsList, pods)
	}
	By(fmt.Sprintf("Waiting for all pods to start %s\n", o.podsList), func() {
		for index, ns := range *o.NSIncluded {
			By(fmt.Sprintf("Waiting for all pods to start %d in namespace %s", index, ns), func() {
				WaitForPods(o.Ctx, o.Client, ns, o.podsList[index])
			})
		}
	})
	return nil
}

func (o *ItemOperations) Verify() error {
	o.Ctx, _ = context.WithTimeout(context.Background(), 60*time.Minute)
	By(fmt.Sprintf("Waiting for all pods to start %s", o.podsList), func() {
		for index, ns := range *o.NSIncluded {
			By(fmt.Sprintf("Waiting for all pods to start %d in namespace %s", index, ns), func() {
				WaitForPods(o.Ctx, o.Client, ns, o.podsList[index])
			})
		}
	})

	if !o.biaOpExtraItem {
		return nil
	}

	for k, ns := range *o.NSIncluded {
		By("Verify Secret added during finalize backed up", func() {
			for i := 0; i <= AsyncBIAPodCount-1; i++ {
				// grab annotations for ns, o.podsList[k][i]
				secretName, err := GetAnnotationFromPod(o.Ctx, o.Client, ns, o.podsList[k][i], AsyncBIAExampleSecretAnnotation)
				Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("failed to find secret annotation for created by BIA operation for pod: %s", o.podsList[k][i]))
				Expect(secretName).To(Not(BeEmpty()))
				_, err = GetSecret(o.Client.ClientGo, ns, secretName)
				Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("failed to find secret %s/%s created by BIA operation for pod: %s", ns, secretName, o.podsList[k][i]))
			}
		})
	}

	return nil
}
