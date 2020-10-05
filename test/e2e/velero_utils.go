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
