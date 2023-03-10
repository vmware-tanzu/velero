package common

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
)

type OsCommandLine struct {
	Cmd  string
	Args []string
}

func GetListByCmdPipes(ctx context.Context, cmdlines []*OsCommandLine) ([]string, error) {
	var buf bytes.Buffer
	var err error
	var cmds []*exec.Cmd
	for _, cmdline := range cmdlines {
		cmd := exec.Command(cmdline.Cmd, cmdline.Args...)
		cmds = append(cmds, cmd)
		fmt.Println(cmd)
	}
	for i := 0; i < len(cmds); i++ {
		if i == len(cmds)-1 {
			break
		}
		cmds[i+1].Stdin, err = cmds[i].StdoutPipe()
		if err != nil {
			return nil, err
		}
	}
	cmds[len(cmds)-1].Stdout = &buf
	for i := len(cmds) - 1; i >= 0; i-- {
		_ = cmds[i].Start()
		if i == 0 {
			_ = cmds[i].Run()
		}
	}
	for i := 1; i < len(cmds); i++ {
		_ = cmds[i].Wait()
	}

	scanner := bufio.NewScanner(&buf)
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

func CMDExecWithOutput(checkCMD *exec.Cmd) (*[]byte, error) {
	stdoutPipe, err := checkCMD.StdoutPipe()
	if err != nil {
		return nil, err
	}

	jsonBuf := make([]byte, 128*1024) // If the YAML is bigger than 64K, there's probably something bad happening

	err = checkCMD.Start()
	if err != nil {
		return nil, err
	}

	bytesRead, err := io.ReadFull(stdoutPipe, jsonBuf)

	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	if bytesRead == len(jsonBuf) {
		return nil, errors.New("yaml returned bigger than max allowed")
	}

	jsonBuf = jsonBuf[0:bytesRead]
	err = checkCMD.Wait()
	if err != nil {
		return nil, err
	}
	return &jsonBuf, err
}
