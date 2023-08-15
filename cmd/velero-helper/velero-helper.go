package main

import (
	"fmt"
	"os"
	"time"
)

const (
	// workingModePause indicates it is for general purpose to hold the pod under running state
	workingModePause = "pause"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "ERROR: at least one argument must be provided, the working mode")
		os.Exit(1)
	}

	switch os.Args[1] {
	case workingModePause:
		time.Sleep(time.Duration(1<<63 - 1))
	default:
		fmt.Fprintln(os.Stderr, "ERROR: wrong working mode provided")
		os.Exit(1)
	}
}
