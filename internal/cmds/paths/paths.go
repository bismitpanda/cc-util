package paths

import (
	"charm.land/lipgloss/v2"
	"github.com/bismitpanda/cc-util/internal/claude"
	"github.com/bismitpanda/cc-util/internal/paths"
	"github.com/bismitpanda/cc-util/internal/ui"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	return &cobra.Command{
		Use:   "paths",
		Short: "Show the files and directories cc-util reads and writes",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			cmdPaths()
		},
	}
}

func cmdPaths() {
	ui.InitStyles()

	entries := []struct {
		label string
		path  string
	}{
		{"Claude config dir", claude.ClaudeDir()},
		{"Claude global file", claude.GlobalFile()},
		{"Claude credentials", claude.CredFile()},
		{"cc-util root", paths.RootDir()},
		{"cc-util accounts", paths.StoreDir()},
		{"cc-util switch log", paths.SwitchesLog()},
		{"cc-util model cache", paths.ModelNamesFile()},
	}

	labelWidth := 0
	for _, e := range entries {
		if w := lipgloss.Width(e.label + ":"); w > labelWidth {
			labelWidth = w
		}
	}

	for _, e := range entries {
		lipgloss.Printf("%s %s\n",
			ui.WhoamiKeyStyle.Width(labelWidth).Align(lipgloss.Right).Render(e.label+":"),
			ui.WhoamiValStyle.Render(e.path),
		)
	}
}
