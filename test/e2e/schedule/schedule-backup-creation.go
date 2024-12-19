package schedule

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/vmware-tanzu/velero/test"
	framework "github.com/vmware-tanzu/velero/test/e2e/test"
	k8sutil "github.com/vmware-tanzu/velero/test/util/k8s"
	veleroutil "github.com/vmware-tanzu/velero/test/util/velero"
)

type ScheduleBackupCreation struct {
	framework.TestCase
	namespace        string
	ScheduleName     string
	ScheduleArgs     []string
	Period           int //Limitation: The unit is minitue only and 60 is divisible by it
	randBackupName   string
	verifyTimes      int
	volume           string
	podName          string
	pvcName          string
	podAnn           map[string]string
	podSleepDuration time.Duration
}

var ScheduleBackupCreationTest func() = framework.TestFunc(&ScheduleBackupCreation{})

func (s *ScheduleBackupCreation) Init() error {
	s.TestCase.Init()
	s.CaseBaseName = "schedule-backup-creation-test" + s.UUIDgen
	s.ScheduleName = "schedule-" + s.CaseBaseName
	s.namespace = s.GetTestCase().CaseBaseName
	s.Period = 3      // Unit is minute
	s.verifyTimes = 5 // More larger verify times more confidence we have
	podSleepDurationStr := "300s"
	s.podSleepDuration, _ = time.ParseDuration(podSleepDurationStr)
	s.TestMsg = &framework.TestMSG{
		Desc:      "Schedule controller wouldn't create a new backup when it still has pending or InProgress backup",
		FailedMSG: "Failed to verify schedule back creation behavior",
		Text:      "Schedule controller wouldn't create a new backup when it still has pending or InProgress backup",
	}
	s.podAnn = map[string]string{
		"pre.hook.backup.velero.io/container": s.podName,
		"pre.hook.backup.velero.io/command":   "[\"sleep\", \"" + podSleepDurationStr + "\"]",
		"pre.hook.backup.velero.io/timeout":   "600s",
	}
	s.volume = "volume-1"
	s.podName = "pod-1"
	s.pvcName = "pvc-1"
	s.ScheduleArgs = []string{
		"--include-namespaces", s.namespace,
		"--schedule=*/" + fmt.Sprintf("%v", s.Period) + " * * * *",
	}
	Expect(s.Period).To(BeNumerically("<", 30))
	return nil
}

func (s *ScheduleBackupCreation) CreateResources() error {
	By(fmt.Sprintf("Create namespace %s", s.namespace), func() {
		Expect(k8sutil.CreateNamespace(s.Ctx, s.Client, s.namespace)).To(Succeed(),
			fmt.Sprintf("Failed to create namespace %s", s.namespace))
	})

	By(fmt.Sprintf("Create pod %s in namespace %s", s.podName, s.namespace), func() {
		_, err := k8sutil.CreatePod(s.Client, s.namespace, s.podName, test.StorageClassName, s.pvcName, []string{s.volume}, nil, s.podAnn)
		Expect(err).To(Succeed())
		err = k8sutil.WaitForPods(s.Ctx, s.Client, s.namespace, []string{s.podName})
		Expect(err).To(Succeed())
	})
	return nil
}

func (s *ScheduleBackupCreation) Backup() error {
	// Wait until the beginning of the given period to create schedule, it will give us
	//   a predictable period to wait for the first scheduled backup, and verify no immediate
	//   scheduled backup was created between schedule creation and first scheduled backup.
	By(fmt.Sprintf("Creating schedule %s ......\n", s.ScheduleName), func() {
		for i := 0; i < s.Period*60/30; i++ {
			time.Sleep(30 * time.Second)
			now := time.Now().Minute()
			triggerNow := now % s.Period
			if triggerNow == 0 {
				Expect(veleroutil.VeleroScheduleCreate(s.Ctx, s.VeleroCfg.VeleroCLI, s.VeleroCfg.VeleroNamespace, s.ScheduleName, s.ScheduleArgs)).To(Succeed(), func() string {
					veleroutil.RunDebug(context.Background(), s.VeleroCfg.VeleroCLI, s.VeleroCfg.VeleroNamespace, "", "")
					return "Fail to create schedule"
				})
				break
			}
		}
	})

	By("Delay one more minute to make sure the new backup was created in the given period", func() {
		time.Sleep(1 * time.Minute)
	})

	By(fmt.Sprintf("Get backups every %d minute, and backups count should increase 1 more step in the same pace\n", s.Period), func() {
		for i := 1; i <= s.verifyTimes; i++ {
			fmt.Printf("Start to sleep %d minute #%d time...\n", s.podSleepDuration, i)
			mi, _ := time.ParseDuration("60s")
			time.Sleep(s.podSleepDuration + mi)
			bMap := make(map[string]string)
			backupsInfo, err := veleroutil.GetScheduledBackupsCreationTime(s.Ctx, s.VeleroCfg.VeleroCLI, "default", s.ScheduleName)
			Expect(err).To(Succeed())
			Expect(backupsInfo).To(HaveLen(i))
			for index, bi := range backupsInfo {
				bList := strings.Split(bi, ",")
				fmt.Printf("Backup %d: %v\n", index, bList)
				bMap[bList[0]] = bList[1]
				_, err := time.Parse("2006-01-02 15:04:05 -0700 MST", bList[1])
				Expect(err).To(Succeed())
			}
			if i == s.verifyTimes-1 {
				backupInfo := backupsInfo[rand.Intn(len(backupsInfo))]
				s.randBackupName = strings.Split(backupInfo, ",")[0]
			}
		}
	})
	return nil
}

func (s *ScheduleBackupCreation) Clean() error {
	if CurrentSpecReport().Failed() && s.VeleroCfg.FailFast {
		fmt.Println("Test case failed and fail fast is enabled. Skip resource clean up.")
	} else {
		Expect(veleroutil.VeleroScheduleDelete(s.Ctx, s.VeleroCfg.VeleroCLI, s.VeleroCfg.VeleroNamespace, s.ScheduleName)).To(Succeed())
		Expect(s.TestCase.Clean()).To(Succeed())
	}

	return nil
}
