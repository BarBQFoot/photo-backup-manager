//go:build !windows

package service

import "os/exec"

func configureAICommand(cmd *exec.Cmd) {}
