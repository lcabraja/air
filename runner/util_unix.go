//go:build unix && !linux

package runner

import (
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func (e *Engine) killCmd(cmd *exec.Cmd) (pid int, err error) {
	pid = cmd.Process.Pid

	if e.config.Build.SendInterrupt {
		e.mainDebug("sending interrupt signal to pid %d", pid)
		// First try to kill the process group
		if err = syscall.Kill(-pid, syscall.SIGINT); err != nil {
			// If process group kill fails, try killing the process directly
			if err = syscall.Kill(pid, syscall.SIGINT); err != nil {
				return
			}
		}
		time.Sleep(e.config.killDelay())
	}

	// Try process group kill first
	e.mainDebug("sending kill signal to pid %d", pid)
	err = syscall.Kill(-pid, syscall.SIGKILL)
	if err != nil {
		// If process group kill fails, try direct process kill
		err = syscall.Kill(pid, syscall.SIGKILL)
	}

	// Wait releases any resources associated with the Process
	e.mainDebug("waiting for process to exit")
	_, _ = cmd.Process.Wait()
	return pid, err
}

func (e *Engine) startCmd(cmd string) (*exec.Cmd, io.ReadCloser, io.ReadCloser, error) {
	e.mainDebug("starting command: %s", cmd)
	c := exec.Command("/bin/sh", "-c", cmd)

	// Set process group for better process management
	c.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		// Ensure child processes are in the same process group
		Pgid: 0,
	}

	stderr, err := c.StderrPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	stdout, err := c.StdoutPipe()
	if err != nil {
		return nil, nil, nil, err
	}

	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	err = c.Start()
	if err != nil {
		return nil, nil, nil, err
	}
	return c, stdout, stderr, nil
}
