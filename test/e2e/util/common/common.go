package common

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
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

	//fmt.Println(&b2)
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

func GetListBy5Pipes(ctx context.Context, cmdline1, cmdline2, cmdline3, cmdline4, cmdline5, cmdline6 OsCommandLine) ([]string, error) {
	var b5 bytes.Buffer
	var errVelero, errAwk error

	c1 := exec.CommandContext(ctx, cmdline1.Cmd, cmdline1.Args...)
	c2 := exec.Command(cmdline2.Cmd, cmdline2.Args...)
	c3 := exec.Command(cmdline3.Cmd, cmdline3.Args...)
	c4 := exec.Command(cmdline4.Cmd, cmdline4.Args...)
	c5 := exec.Command(cmdline5.Cmd, cmdline5.Args...)
	c6 := exec.Command(cmdline6.Cmd, cmdline6.Args...)
	fmt.Println(c1)
	fmt.Println(c2)
	fmt.Println(c3)
	fmt.Println(c4)
	fmt.Println(c5)
	fmt.Println(c6)
	c2.Stdin, errVelero = c1.StdoutPipe()
	if errVelero != nil {
		return nil, errVelero
	}
	c3.Stdin, errAwk = c2.StdoutPipe()
	if errAwk != nil {
		return nil, errAwk
	}
	c4.Stdin, errAwk = c3.StdoutPipe()
	if errAwk != nil {
		return nil, errAwk
	}
	c5.Stdin, errAwk = c4.StdoutPipe()
	if errAwk != nil {
		return nil, errAwk
	}
	c6.Stdin, errAwk = c5.StdoutPipe()
	if errAwk != nil {
		return nil, errAwk
	}
	c6.Stdout = &b5

	_ = c6.Start()
	_ = c5.Start()
	_ = c4.Start()
	_ = c3.Start()
	_ = c2.Start()
	_ = c1.Run()
	_ = c2.Wait()
	_ = c3.Wait()
	_ = c4.Wait()
	_ = c5.Wait()
	_ = c6.Wait()

	//fmt.Println(&b2)
	scanner := bufio.NewScanner(&b5)
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
