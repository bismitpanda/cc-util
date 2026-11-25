//go:build windows

package browser

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

func run(args []string, shell string, env []string) error {
	var cmd *exec.Cmd
	if shell != "" {
		comspec := os.Getenv("COMSPEC")
		if comspec == "" {
			comspec = "cmd.exe"
		}
		cmd = exec.Command(comspec)
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: fmt.Sprintf(`"%s" /d /s /c "%s"`, comspec, shell)}
	} else {
		path, err := exec.LookPath(args[0])
		if err != nil {
			return fmt.Errorf("could not find %s: %w", args[0], err)
		}
		cmd = exec.Command(path, args[1:]...)
	}
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	signal.Ignore(os.Interrupt)
	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	if err != nil {
		return err
	}
	os.Exit(0)
	return nil
}
