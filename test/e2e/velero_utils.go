package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"

	"github.com/pkg/errors"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

func CheckBackupPhase(ctx context.Context, veleroCLI string, backupName string, expectedPhase v1.BackupPhase) error {
	checkCMD := exec.CommandContext(ctx, veleroCLI, "backup", "get", "-o", "json", backupName)
	fmt.Printf("get backup cmd =%v\n", checkCMD)
	stdoutPipe, err := checkCMD.StdoutPipe()
	if err != nil {
		return err
	}

	jsonBuf := make([]byte, 16*1024) // If the YAML is bigger than 16K, there's probably something bad happening

	err = checkCMD.Start()
	if err != nil {
		return err
	}

	bytesRead, err := io.ReadFull(stdoutPipe, jsonBuf)

	if err != nil && err != io.ErrUnexpectedEOF {
		return err
	}
	if bytesRead == len(jsonBuf) {
		return errors.New("yaml returned bigger than max allowed")
	}

	jsonBuf = jsonBuf[0:bytesRead]
	err = checkCMD.Wait()
	if err != nil {
		return err
	}
	backup := v1.Backup{}
	err = json.Unmarshal(jsonBuf, &backup)
	if err != nil {
		return err
	}
	if backup.Status.Phase != expectedPhase {
		return errors.Errorf("Unexpected backup phase got %s, expecting %s", backup.Status.Phase, expectedPhase)
	}
	return nil
}

func CheckRestorePhase(ctx context.Context, veleroCLI string, restoreName string, expectedPhase v1.RestorePhase) error {
	checkCMD := exec.CommandContext(ctx, veleroCLI, "restore", "get", "-o", "json", restoreName)
	fmt.Printf("get restore cmd =%v\n", checkCMD)
	stdoutPipe, err := checkCMD.StdoutPipe()
	if err != nil {
		return err
	}

	jsonBuf := make([]byte, 16*1024) // If the YAML is bigger than 16K, there's probably something bad happening

	err = checkCMD.Start()
	if err != nil {
		return err
	}

	bytesRead, err := io.ReadFull(stdoutPipe, jsonBuf)

	if err != nil && err != io.ErrUnexpectedEOF {
		return err
	}
	if bytesRead == len(jsonBuf) {
		return errors.New("yaml returned bigger than max allowed")
	}

	jsonBuf = jsonBuf[0:bytesRead]
	err = checkCMD.Wait()
	if err != nil {
		return err
	}
	restore := v1.Restore{}
	err = json.Unmarshal(jsonBuf, &restore)
	if err != nil {
		return err
	}
	if restore.Status.Phase != expectedPhase {
		return errors.Errorf("Unexpected restore phase got %s, expecting %s", restore.Status.Phase, expectedPhase)
	}
	return nil
}

func BackupNamespace(ctx context.Context, veleroCLI string, backupName string, namespace string) error {
	backupCmd := exec.CommandContext(ctx, veleroCLI, "create", "backup", backupName, "--include-namespaces", namespace,
		"--default-volumes-to-restic", "--wait")
	fmt.Printf("backup cmd =%v\n", backupCmd)
	err := backupCmd.Run()
	if err != nil {
		return err
	}
	return CheckBackupPhase(ctx, veleroCLI, backupName, v1.BackupPhaseCompleted)
}

func RestoreNamespace(ctx context.Context, veleroCLI string, restoreName string, backupName string) error {
	restoreCmd := exec.CommandContext(ctx, veleroCLI, "create", "restore", restoreName, "--from-backup", backupName, "--wait")
	fmt.Printf("restore cmd =%v\n", restoreCmd)
	err := restoreCmd.Run()
	if err != nil {
		return err
	}
	return CheckRestorePhase(ctx, veleroCLI, restoreName, v1.RestorePhaseCompleted)
}
