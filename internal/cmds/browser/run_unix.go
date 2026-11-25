//go:build !windows

package browser

import (
	"fmt"
	"os/exec"
	"syscall"
)

func run(args []string, shell string, env []string) error {
	if shell != "" {
		args = []string{"/bin/sh", "-c", shell}
	}
	path, err := exec.LookPath(args[0])
	if err != nil {
		return fmt.Errorf("could not find %s: %w", args[0], err)
	}
	return syscall.Exec(path, args, env)
}
