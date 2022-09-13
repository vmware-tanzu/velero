package common

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"github.com/pkg/errors"

	veleroexec "github.com/vmware-tanzu/velero/pkg/util/exec"
)

type OsCommandLine struct {
	Cmd  string
	Args []string
}

func GetListBy2Pipes(ctx context.Context, cmdline1, cmdline2, cmdline3 OsCommandLine) ([]string, error) {
	var b2 bytes.Buffer
	var errVelero, errAwk error

	c1 := exec.CommandContext(ctx, cmdline1.Cmd, cmdline1.Args...)
	c2 := exec.Command(cmdline2.Cmd, cmdline2.Args...)
	c3 := exec.Command(cmdline3.Cmd, cmdline3.Args...)
	fmt.Println(c1)
	fmt.Println(c2)
	fmt.Println(c3)
	c2.Stdin, errVelero = c1.StdoutPipe()
	if errVelero != nil {
		return nil, errVelero
	}
	c3.Stdin, errAwk = c2.StdoutPipe()
	if errAwk != nil {
		return nil, errAwk
	}
	c3.Stdout = &b2
	_ = c3.Start()
	_ = c2.Start()
	_ = c1.Run()
	_ = c2.Wait()
	_ = c3.Wait()

	fmt.Println(&b2)
	scanner := bufio.NewScanner(&b2)
	var ret []string
	for scanner.Scan() {
		fmt.Printf("line: %s\n", scanner.Text())
		ret = append(ret, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return ret, nil
}

func KubectlApplyFile(ctx context.Context, yaml string) error {
	fmt.Printf("Kube apply file %s.\n", yaml)
	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", yaml)

	_, stderr, err := veleroexec.RunCommand(cmd)
	if err != nil {
		return errors.Wrap(err, stderr)
	}

	return nil
}
