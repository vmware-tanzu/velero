package schedule

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/vmware-tanzu/velero/test/e2e"
	. "github.com/vmware-tanzu/velero/test/e2e/test"
	. "github.com/vmware-tanzu/velero/test/e2e/util/k8s"
	. "github.com/vmware-tanzu/velero/test/e2e/util/velero"
)

type ScheduleBackup struct {
	TestCase
	ScheduleName   string
	ScheduleArgs   []string
	Period         int //Limitation: The unit is minitue only and 60 is divisible by it
	randBackupName string
	verifyTimes    int
}

var ScheduleBackupTest func() = TestFunc(&ScheduleBackup{TestCase: TestCase{NSBaseName: "schedule-test"}})

func (n *ScheduleBackup) Init() error {
	//n.Client = TestClientInstance
	n.VeleroCfg = VeleroCfg
	n.Client = *n.VeleroCfg.ClientToInstallVelero
	n.Period = 3      // Unit is minute
	n.verifyTimes = 5 // More verify times more confidence
	n.TestMsg = &TestMSG{
		Desc:      "Set up a scheduled backup defined by a Cron expression",
		FailedMSG: "Failed to schedule a backup",
		Text:      "should backup periodly according to the schedule",
	}
	return nil
}

func (n *ScheduleBackup) StartRun() error {
	n.NSIncluded = &[]string{fmt.Sprintf("%s-%s", n.NSBaseName, "ns")}
	n.ScheduleName = n.ScheduleName + "schedule-" + UUIDgen.String()
	n.RestoreName = n.RestoreName + "restore-ns-mapping-" + UUIDgen.String()

	n.ScheduleArgs = []string{
		"--include-namespaces", strings.Join(*n.NSIncluded, ","),
		"--schedule=*/" + fmt.Sprintf("%v", n.Period) + " * * * *",
	}
	Expect(n.Period < 30).To(Equal(true))
	return nil
}
func (n *ScheduleBackup) CreateResources() error {
	n.Ctx, _ = context.WithTimeout(context.Background(), 60*time.Minute)
	for _, ns := range *n.NSIncluded {
		By(fmt.Sprintf("Creating namespaces %s ......\n", ns), func() {
			Expect(CreateNamespace(n.Ctx, n.Client, ns)).To(Succeed(), fmt.Sprintf("Failed to create namespace %s", ns))
		})
		configmaptName := n.NSBaseName
		fmt.Printf("Creating configmap %s in namespaces ...%s\n", configmaptName, ns)
		_, err := CreateConfigMap(n.Client.ClientGo, ns, configmaptName, nil)
		Expect(err).To(Succeed(), fmt.Sprintf("failed to create configmap in the namespace %q", ns))
		Expect(WaitForConfigMapComplete(n.Client.ClientGo, ns, configmaptName)).To(Succeed(),
			fmt.Sprintf("ailed to ensure secret completion in namespace: %q", ns))
	}
	return nil
}

func (n *ScheduleBackup) Backup() error {
	// Wait until the beginning of the given period to create schedule, it will give us
	//   a predictable period to wait for the first scheduled backup, and verify no immediate
	//   scheduled backup was created between schedule creation and first scheduled backup.
	By(fmt.Sprintf("Creating schedule %s ......\n", n.ScheduleName), func() {
		for i := 0; i < n.Period*60/30; i++ {
			time.Sleep(30 * time.Second)
			now := time.Now().Minute()
			triggerNow := now % n.Period
			if triggerNow == 0 {
				Expect(VeleroScheduleCreate(n.Ctx, VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace, n.ScheduleName, n.ScheduleArgs)).To(Succeed(), func() string {
					RunDebug(context.Background(), VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace, "", "")
					return "Fail to restore workload"
				})
				break
			}
		}
	})
	return nil
}
func (n *ScheduleBackup) Destroy() error {
	By(fmt.Sprintf("Schedule %s is created without any delay\n", n.ScheduleName), func() {
		creationTimestamp, err := GetSchedule(context.Background(), VeleroCfg.VeleroNamespace, n.ScheduleName)
		Expect(err).To(Succeed())

		creationTime, err := time.Parse(time.RFC3339, strings.Replace(creationTimestamp, "'", "", -1))
		Expect(err).To(Succeed())
		fmt.Printf("Schedule %s created at %s\n", n.ScheduleName, creationTime)
		now := time.Now()
		diff := creationTime.Sub(now)
		Expect(diff.Minutes() < 1).To(Equal(true))
	})

	By(fmt.Sprintf("No immediate backup is created by schedule %s\n", n.ScheduleName), func() {
		for i := 0; i < n.Period; i++ {
			time.Sleep(1 * time.Minute)
			now := time.Now()
			fmt.Printf("Get backup for #%d time at %v\n", i, now)
			//Ignore the last minute in the period avoiding met the 1st backup by schedule
			if i != n.Period-1 {
				backupsInfo, err := GetScheduledBackupsCreationTime(context.Background(), VeleroCfg.VeleroCLI, "default", n.ScheduleName)
				Expect(err).To(Succeed())
				Expect(len(backupsInfo) == 0).To(Equal(true))
			}
		}
	})

	By("Delay one more minute to make sure the new backup was created in the given period", func() {
		time.Sleep(1 * time.Minute)
	})

	By(fmt.Sprintf("Get backups every %d minute, and backups count should increase 1 more step in the same pace\n", n.Period), func() {
		for i := 0; i < n.verifyTimes; i++ {
			fmt.Printf("Start to sleep %d minute #%d time...\n", n.Period, i+1)
			time.Sleep(time.Duration(n.Period) * time.Minute)
			bMap := make(map[string]string)
			backupsInfo, err := GetScheduledBackupsCreationTime(context.Background(), VeleroCfg.VeleroCLI, "default", n.ScheduleName)
			Expect(err).To(Succeed())
			Expect(len(backupsInfo) == i+2).To(Equal(true))
			for index, bi := range backupsInfo {
				bList := strings.Split(bi, ",")
				fmt.Printf("Backup %d: %v\n", index, bList)
				bMap[bList[0]] = bList[1]
				_, err := time.Parse("2006-01-02 15:04:05 -0700 MST", bList[1])
				Expect(err).To(Succeed())
			}
			if i == n.verifyTimes-1 {
				backupInfo := backupsInfo[rand.Intn(len(backupsInfo))]
				n.randBackupName = strings.Split(backupInfo, ",")[0]
			}
		}
	})

	n.BackupName = strings.Replace(n.randBackupName, " ", "", -1)

	By("Delete all namespaces", func() {
		Expect(CleanupNamespacesWithPoll(n.Ctx, n.Client, n.NSBaseName)).To(Succeed(), "Could cleanup retrieve namespaces")
	})

	n.RestoreArgs = []string{
		"create", "--namespace", VeleroCfg.VeleroNamespace, "restore", n.RestoreName,
		"--from-backup", n.BackupName,
		"--wait",
	}

	backupsInfo, err := GetScheduledBackupsCreationTime(context.Background(), VeleroCfg.VeleroCLI, "default", n.ScheduleName)
	Expect(err).To(Succeed(), fmt.Sprintf("Fail to get backups from schedule %s", n.ScheduleName))
	fmt.Println(backupsInfo)
	backupCount := len(backupsInfo)

	By(fmt.Sprintf("Pause schedule %s ......\n", n.ScheduleName), func() {
		Expect(VeleroSchedulePause(n.Ctx, VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace, n.ScheduleName)).To(Succeed(), func() string {
			RunDebug(context.Background(), VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace, "", "")
			return "Fail to restore workload"
		})
	})

	periodCount := 3
	sleepDuration := time.Duration(n.Period*periodCount) * time.Minute
	By(fmt.Sprintf("Sleep for %s ......\n", sleepDuration), func() {
		time.Sleep(sleepDuration)
	})

	backupsInfo, err = GetScheduledBackupsCreationTime(context.Background(), VeleroCfg.VeleroCLI, "default", n.ScheduleName)
	Expect(err).To(Succeed(), fmt.Sprintf("Fail to get backups from schedule %s", n.ScheduleName))

	backupCountPostPause := len(backupsInfo)
	fmt.Printf("After pause, backkups count is %d\n", backupCountPostPause)

	By(fmt.Sprintf("Verify no new backups from %s ......\n", n.ScheduleName), func() {
		Expect(backupCountPostPause == backupCount).To(Equal(true))
	})

	By(fmt.Sprintf("Unpause schedule %s ......\n", n.ScheduleName), func() {
		Expect(VeleroScheduleUnpause(n.Ctx, VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace, n.ScheduleName)).To(Succeed(), func() string {
			RunDebug(context.Background(), VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace, "", "")
			return "Fail to unpause schedule"
		})
	})

	By(fmt.Sprintf("Sleep for %s ......\n", sleepDuration), func() {
		time.Sleep(sleepDuration)
	})

	backupsInfo, err = GetScheduledBackupsCreationTime(context.Background(), VeleroCfg.VeleroCLI, "default", n.ScheduleName)
	Expect(err).To(Succeed(), fmt.Sprintf("Fail to get backups from schedule %s", n.ScheduleName))
	fmt.Println(backupsInfo)
	backupCountPostUnpause := len(backupsInfo)
	fmt.Printf("After unpause, backkups count is %d\n", backupCountPostUnpause)
	By(fmt.Sprintf("Verify no new backups by schedule %s ......\n", n.ScheduleName), func() {
		Expect(backupCountPostUnpause-backupCount >= periodCount-1).To(Equal(true))
	})
	return nil
}

func (n *ScheduleBackup) Verify() error {
	By("Namespaces were restored", func() {
		for _, ns := range *n.NSIncluded {
			configmap, err := GetConfigmap(n.Client.ClientGo, ns, n.NSBaseName)
			fmt.Printf("Restored configmap is %v\n", configmap)
			Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("failed to list configmap in namespace: %q\n", ns))
		}

	})
	return nil
}
