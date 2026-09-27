package projects

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/bismitpanda/cc-util/internal/claude"
	"github.com/bismitpanda/cc-util/internal/cli"
	"github.com/bismitpanda/cc-util/internal/cmds/accounts"
	"github.com/bismitpanda/cc-util/internal/ui"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	var long bool
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "List Claude Code project folders",
		Long:  "Lists the project folders saved in ~/.claude.json. --long adds the last session id, models, lines, cost, and MCP server count.",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			cmdProjects(long)
		},
	}
	cmd.Flags().BoolVar(&long, "long", false, "Also show the last session id, models, lines changed, cost, and MCP server count")
	return cmd
}

func cmdProjects(long bool) {
	list, err := claude.ListProjects()
	if err != nil {
		ui.Fatalf("%v", err)
	}
	if len(list) == 0 {
		if cli.JSON {
			ui.PrintJSON([]projectJSON{})
			return
		}
		ui.PrintMuted("(no projects)")
		return
	}

	names := map[string]string{}
	if long {
		var ids []string
		for _, p := range list {
			ids = append(ids, p.Models...)
		}
		token := ""
		if name, ok := accounts.ActiveSavedAccountName(); ok {
			token, _ = accounts.EnsureAccountAccessToken(name)
		}
		names, err = claude.LoadModelNames(token, ids)
		if err != nil && !cli.JSON {
			ui.PrintMuted(fmt.Sprintf("could not refresh model names (%v); using the cached list", err))
		}
	}

	if cli.JSON {
		out := make([]projectJSON, len(list))
		for i, p := range list {
			out[i] = projectJSON{
				Path:          p.Path,
				OnDisk:        p.OnDisk,
				Trusted:       p.Trusted,
				LastSessionID: p.LastSessionID,
				LinesAdded:    p.LinesAdded,
				LinesRemoved:  p.LinesRemoved,
				Cost:          p.LastCost,
				MCPServers:    p.MCPServers,
			}
			if !p.LastStart.IsZero() {
				out[i].LastStart = p.LastStart.Format(time.RFC3339)
			}
			if long {
				out[i].Models = modelNamesList(p.Models, names)
			} else {
				out[i].Models = p.Models
			}
		}
		ui.PrintJSON(out)
		return
	}

	rows := make([][]string, len(list))
	for i, p := range list {
		row := []string{
			p.Path,
			yesNo(p.OnDisk),
			yesNo(p.Trusted),
			formatSession(p.LastSessionID, p.LastStart, long),
		}
		if long {
			row = append(row,
				formatModels(p.Models, names),
				formatLines(p),
				formatCost(p.LastCost),
				strconv.Itoa(p.MCPServers),
			)
		}
		rows[i] = row
	}

	headers := []string{"Folder", "On disk", "Trusted", "Last session"}
	if long {
		headers = append(headers, "Models", "Lines", "Cost", "MCP")
	}

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

type projectJSON struct {
	Path          string   `json:"path"`
	OnDisk        bool     `json:"onDisk"`
	Trusted       bool     `json:"trusted"`
	LastStart     string   `json:"lastStart,omitempty"`
	LastSessionID string   `json:"lastSessionId,omitempty"`
	Models        []string `json:"models,omitempty"`
	LinesAdded    int      `json:"linesAdded"`
	LinesRemoved  int      `json:"linesRemoved"`
	Cost          *float64 `json:"cost,omitempty"`
	MCPServers    int      `json:"mcpServers"`
}

func modelNamesList(models []string, names map[string]string) []string {
	var out []string
	seen := map[string]bool{}
	for _, id := range models {
		name, ok := claude.DisplayModel(id, names)
		if !ok || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

func yesNo(v bool) string {
	if v {
		return "✅"
	}
	return "❌"
}

func formatLastStart(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Local().Format("Jan 2, 2006 3:04 PM")
}

func formatSession(id string, lastStart time.Time, long bool) string {
	date := formatLastStart(lastStart)
	if !long || id == "" {
		return date
	}
	return fmt.Sprintf("%s (%s)", id, date)
}

func formatLines(p claude.Project) string {
	if p.LastStart.IsZero() && p.LinesAdded == 0 && p.LinesRemoved == 0 {
		return "—"
	}
	return fmt.Sprintf("+%d -%d", p.LinesAdded, p.LinesRemoved)
}

func formatCost(cost *float64) string {
	if cost == nil {
		return "—"
	}
	return fmt.Sprintf("$%.2f", *cost)
}

func formatModels(models []string, names map[string]string) string {
	if len(models) == 0 {
		return "—"
	}
	var shown []string
	seen := map[string]bool{}
	for _, id := range models {
		name, ok := claude.DisplayModel(id, names)
		if !ok || seen[name] {
			continue
		}
		seen[name] = true
		shown = append(shown, name)
	}
	if len(shown) == 0 {
		return "—"
	}
	return strings.Join(shown, ", ")
}
