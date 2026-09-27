package memory

import (
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/bismitpanda/cc-util/internal/claude"
	"github.com/bismitpanda/cc-util/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func projectPathCompletions() ([]string, cobra.ShellCompDirective) {
	list, err := claude.ListProjects()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	paths := make([]string, len(list))
	for i, p := range list {
		paths[i] = p.Path
	}
	return paths, cobra.ShellCompDirectiveNoFileComp
}

func memoryNameCompletions(project string) ([]string, cobra.ShellCompDirective) {
	list, err := claude.ListMemories(project)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	names := make([]string, len(list))
	for i, m := range list {
		names[i] = m.Name
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

func completeListArgs(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return projectPathCompletions()
}

func completeShowArgs(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	switch len(args) {
	case 0:
		return projectPathCompletions()
	case 1:
		return memoryNameCompletions(args[0])
	default:
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "memory",
		Aliases: []string{"mem"},
		Short:   "Claude Code auto memories",
		Long:    "Lists and shows auto memories for the project folders in ~/.claude.json.",
	}
	cmd.AddCommand(listCommand(), showCommand(), issuesCommand())
	return cmd
}

func listCommand() *cobra.Command {
	var long, all bool
	cmd := &cobra.Command{
		Use:     "list [project]",
		Aliases: []string{"ls"},
		Short:   "List auto memories",
		Long:              "Lists auto memories for the current project folder. Pass a project path or unique folder name to list that project. --all prints one table per folder. --long adds the description from the memory file.",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeListArgs,
		Run: func(_ *cobra.Command, args []string) {
			if all && len(args) > 0 {
				ui.Fatalf("cannot combine --all with a project")
			}
			project := ""
			if len(args) > 0 {
				project = args[0]
			} else if !all {
				var err error
				project, err = claude.CurrentProject()
				if err != nil {
					ui.Fatalf("%v", err)
				}
			}
			cmdMemory(project, long)
		},
	}
	cmd.Flags().BoolVar(&long, "long", false, "Show the description from the memory file")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "List memories for every project")
	return cmd
}

func showCommand() *cobra.Command {
	return &cobra.Command{
		Use:               "show <project> <file>",
		Short:             "Print one memory",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeShowArgs,
		Run: func(_ *cobra.Command, args []string) {
			cmdShow(args[0], args[1])
		},
	}
}

func issuesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "issues",
		Short: "Show memories missing a file, or files missing from the index",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			cmdIssues()
		},
	}
}

func cmdMemory(project string, long bool) {
	list, err := claude.ListMemories(project)
	if err != nil {
		ui.Fatalf("%v", err)
	}
	if len(list) == 0 {
		ui.PrintMuted("(no memories)")
		return
	}
	headers := []string{"Memory", "Type"}
	if long {
		headers = append(headers, "Description")
	}
	var current string
	var rows [][]string
	flush := func() {
		if len(rows) == 0 {
			return
		}
		fmt.Println(current)
		printTable(headers, rows)
		fmt.Println()
		rows = nil
	}
	for _, item := range list {
		if item.Project != current {
			flush()
			current = item.Project
		}
		typ := item.Type
		if typ == "" {
			typ = "—"
		}
		row := []string{item.Name, typ}
		if long {
			desc := item.Description
			if desc == "" {
				desc = "—"
			}
			row = append(row, desc)
		}
		rows = append(rows, row)
	}
	flush()
}

func formatModified(t time.Time) string {
	return t.Local().Format("Jan 2, 2006 3:04 PM")
}

func cmdShow(project, name string) {
	page, err := claude.ReadMemory(project, name)
	if err != nil {
		ui.Fatalf("%v", err)
	}
	ui.InitStyles()
	if page.Title != "" {
		lipgloss.Println(ui.TitleStyle.Render(page.Title))
	}
	var meta []string
	if page.Type != "" {
		meta = append(meta, ui.LabelStyle.Render("Type: ")+cases.Title(language.English).String(page.Type))
	}
	if !page.Modified.IsZero() {
		meta = append(meta, ui.LabelStyle.Render("Modified: ")+formatModified(page.Modified))
	}
	if len(meta) > 0 {
		lipgloss.Println(strings.Join(meta, "   "))
	}
	if page.Description != "" {
		fmt.Println(page.Description)
	}
	if page.Title != "" || len(meta) > 0 || page.Description != "" {
		fmt.Println()
	}
	if page.Body == "" {
		return
	}
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width < 1 {
		width = 80
	}
	style := styles.LightStyle
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		style = styles.NoTTYStyle
	} else if lipgloss.HasDarkBackground(os.Stdin, os.Stdout) {
		style = styles.DarkStyle
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(style),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		ui.Fatalf("%v", err)
	}
	rendered, err := renderer.Render(page.Body)
	if err != nil {
		ui.Fatalf("%v", err)
	}
	fmt.Print(rendered)
}

func cmdIssues() {
	list, err := claude.ListMemoryIssues()
	if err != nil {
		ui.Fatalf("%v", err)
	}
	if len(list) == 0 {
		ui.PrintMuted("(no memory issues)")
		return
	}
	ui.InitStyles()
	mark := lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render("⚠️")
	project := ""
	for _, item := range list {
		if item.Project != project {
			if project != "" {
				fmt.Println()
			}
			project = item.Project
			fmt.Println(project)
		}
		fmt.Printf("  %s %s\n", mark, issueText(item))
	}
}

func issueText(item claude.MemoryIssue) string {
	if item.Kind == "indexed, missing file" {
		return item.Name + " is in the index but the file is missing"
	}
	return item.Name + " is not in the index"
}

func printTable(headers []string, rows [][]string) {
	ui.InitStyles()
	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(ui.MutedStyle).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return ui.LabelStyle.Bold(true).Padding(0, 1)
			}
			if col == 0 {
				return ui.AccountStyle.Padding(0, 1)
			}
			return ui.WhoamiValStyle.Padding(0, 1)
		}).
		Headers(headers...).
		Rows(rows...)
	lipgloss.Println(t)
}
