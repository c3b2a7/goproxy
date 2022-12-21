package process

import (
	"fmt"
	"github.com/c3b2a7/goproxy/common/hotupdate"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

var (
	subProcess *exec.Cmd
	exitCh     = make(chan struct{})
)

func Start(daemon, forever bool, startService func() error) error {
	if daemon {
		fmt.Printf("[*] Daemon running in PID: %d PPID: %d\n", os.Getpid(), os.Getppid())
		fork(stripSlice(os.Args, "--daemon"), os.Stdout, os.Stderr)
		os.Exit(0)
	} else if forever {
		fmt.Printf("[*] Forever running in PID: %d PPID: %d\n", os.Getpid(), os.Getppid())
		go func() {
			for {
				subProcess = fork(stripSlice(os.Args, "--forever"), os.Stdout, os.Stderr)
				subProcess.Wait()
				select {
				case <-exitCh:
					return
				default:
				}
			}
		}()
	} else {
		fmt.Printf("[*] Service running in PID: %d PPID: %d ARG: %s\n", os.Getpid(), os.Getppid(), os.Args)
		if err := startService(); err != nil {
			return err
		}
		hotupdate.StartService(func(newVersion string) {
			fmt.Printf("\n[*] New version(%s) avaliable, restart services for update...\n", newVersion)
			os.Exit(0)
		})
	}
	return nil
}

func Restart() {
	rescheduleSubProcess(false)
}

func Kill() {
	rescheduleSubProcess(true)
}

func rescheduleSubProcess(exit bool) {
	if subProcess != nil {
		waitCh := make(chan struct{})
		go func() {
			subProcess.Wait()
			close(waitCh)
		}()

		if exit {
			close(exitCh)
		}
		subProcess.Process.Signal(syscall.SIGTERM)
		select {
		case <-waitCh:
		case <-time.After(3 * time.Second):
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
