//go:build !windows

package service

import "os/exec"

func configureAICommand(cmd *exec.Cmd) {}

func terminateAICommand(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
