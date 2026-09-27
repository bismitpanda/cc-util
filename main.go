package main

import (
	"context"
	"os"
	"strings"

	"charm.land/fang/v2"
	"github.com/bismitpanda/cc-util/internal/cli"
	"github.com/bismitpanda/cc-util/internal/cmds/accounts"
	"github.com/bismitpanda/cc-util/internal/cmds/projects"
	"github.com/bismitpanda/cc-util/internal/ui"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	bin     = "cc-util"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           bin,
		Short:         "Utilities for Claude Code",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			name := cmd.Name()
			if name == "completion" || name == "help" || name == "man" ||
				strings.HasPrefix(name, "__") {
				return
			}
			ui.InitStyles()
			accounts.EnsureSetup()
		},
	}
	cli.Bin = bin
	root.AddCommand(accounts.Command(), projects.Command())
	return root
}

func main() {
	if err := fang.Execute(
		context.Background(),
		newRootCmd(),
		fang.WithVersion(version),
		fang.WithNotifySignal(os.Interrupt),
	); err != nil {
		os.Exit(1)
	}
}
