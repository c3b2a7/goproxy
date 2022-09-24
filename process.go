package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

var (
	subProcess *exec.Cmd
	extCh      = make(chan struct{})
)

func ForkSubProcess(daemon, forever bool) bool {
	if daemon {
		fmt.Printf("[*] Daemon running in PID: %d PPID: %d\n", os.Getpid(), os.Getppid())
		fork(stripSlice(os.Args, "--daemon"), os.Stdout, os.Stderr)
		os.Exit(0)
		return true
	} else if forever {
		fmt.Printf("[*] Forever running in PID: %d PPID: %d\n", os.Getpid(), os.Getppid())
		go func() {
			for {
				subProcess = fork(stripSlice(os.Args, "--forever"), os.Stdout, os.Stderr)
				subProcess.Wait()
				timeout := time.After(1 * time.Second)
				select {
				case <-timeout:
				case <-extCh:
					return
				}
			}
		}()
		return true
	}
	fmt.Printf("[*] Service running in PID: %d PPID: %d ARG: %s\n", os.Getpid(), os.Getppid(), os.Args)
	return false
}

func KillSubProcess() {
	if subProcess != nil {
		waitCh := make(chan struct{})
		go func() {
			subProcess.Wait()
			close(waitCh)
		}()

		close(extCh)
		subProcess.Process.Signal(syscall.SIGTERM)

		timeout := time.After(3 * time.Second)
		select {
		case <-waitCh:
		case <-timeout:
			subProcess.Process.Kill()
		}
	}
}

func fork(args []string, stdout, stderr io.Writer) *exec.Cmd {
	cmd := &exec.Cmd{
		Path:   args[0],
		Args:   args,
		Stdout: stdout,
		Stderr: stderr,
	}
	err := cmd.Start()
	if err != nil {
		fmt.Printf("[-] Error: %s\n", err)
	}
	return cmd
}

func stripSlice(slice []string, element string) []string {
	for i := 0; i < len(slice); {
		if slice[i] == element && i != len(slice)-1 {
			slice = append(slice[:i], slice[i+1:]...)
		} else if slice[i] == element && i == len(slice)-1 {
			slice = slice[:i]
		} else {
			i++
		}
	}
	return slice
}
