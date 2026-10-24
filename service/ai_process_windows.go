package service

import (
	"context"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

func configureAICommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}

func terminateAICommand(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// A Windows venv Python can launch the real interpreter as a child process.
	kill := exec.CommandContext(ctx, "taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
	configureAICommand(kill)
	_ = kill.Run()
	_ = cmd.Process.Kill()
}
