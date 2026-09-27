package ui

import (
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"golang.org/x/term"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	stylesReady bool

	TitleStyle       lipgloss.Style
	SuccessStyle     lipgloss.Style
	MutedStyle       lipgloss.Style
	ErrorStyle       lipgloss.Style
	AccountStyle     lipgloss.Style
	LabelStyle       lipgloss.Style
	BarFillColor     = lipgloss.Color("42")
	BarEmptyColor    = lipgloss.Color("238")
	CriticalBarColor = lipgloss.Color("203")
	ActiveBarColor   = lipgloss.Color("214")
	PaceBlack        = lipgloss.Color("#000000")
	PaceWhite        = lipgloss.Color("#ffffff")

	BarFillStyle     lipgloss.Style
	BarEmptyStyle    lipgloss.Style
	CriticalBarStyle lipgloss.Style
	ActiveBarStyle   lipgloss.Style
	PaceMarkStyle    lipgloss.Style
	WhoamiKeyStyle   lipgloss.Style
	WhoamiValStyle   lipgloss.Style
	helpTitleStyle   lipgloss.Style
	helpCmdStyle     lipgloss.Style
	helpArgStyle     lipgloss.Style
)

func InitStyles() {
	if stylesReady {
		return
	}
	stylesReady = true

	if !term.IsTerminal(int(os.Stdout.Fd())) {
		lipgloss.Writer = colorprofile.NewWriter(os.Stdout, []string{"TERM=dumb"})
	}

	TitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	SuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	MutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	ErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	AccountStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("117"))
	LabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	BarFillStyle = lipgloss.NewStyle().Foreground(BarFillColor)
	BarEmptyStyle = lipgloss.NewStyle().Foreground(BarEmptyColor)
	CriticalBarStyle = lipgloss.NewStyle().Foreground(CriticalBarColor)
	ActiveBarStyle = lipgloss.NewStyle().Foreground(ActiveBarColor)
	PaceMarkStyle = lipgloss.NewStyle().Foreground(PaceBlack)
	WhoamiKeyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	WhoamiValStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	helpTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	helpCmdStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	helpArgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
}

func PrintMuted(msg string) {
	InitStyles()
	lipgloss.Println(MutedStyle.Render(msg))
}

func PrintSuccess(msg string) {
	InitStyles()
	lipgloss.Println(SuccessStyle.Render(msg))
}

func PrintError(name, msg string) {
	InitStyles()
	if name != "" {
		lipgloss.Fprintf(os.Stderr, "%s: %s\n", AccountStyle.Render(name), ErrorStyle.Render(msg))
		return
	}
	lipgloss.Fprintln(os.Stderr, ErrorStyle.Render(msg))
}

func Fatalf(format string, args ...any) {
	PrintError("", fmt.Sprintf(format, args...))
	os.Exit(1)
}

var whoamiFields = []struct {
	key   string
	label string
}{
	{"displayName", "Name"},
	{"emailAddress", "Email"},
	{"organizationName", "Organization"},
	{"organizationType", "Type"},
}

func WhoamiFieldValue(v any) (string, bool) {
	if v == nil {
		return "", false
	}
	switch val := v.(type) {
	case string:
		s := strings.TrimSpace(val)
		return s, s != ""
	default:
		return "", false
	}
}

func FormatWhoamiField(key, value string) string {
	if key == "organizationType" || key == "billingType" {
		return cases.Title(language.English).String(strings.ReplaceAll(value, "_", " "))
	}
	return value
}

func PrintWhoami(oauth any) {
	InitStyles()
	if oauth == nil {
		PrintMuted("(no active account)")
		return
	}
	m, ok := oauth.(map[string]any)
	if !ok || len(m) == 0 {
		PrintMuted("(no active account)")
		return
	}

	var lines []struct {
		label string
		value string
	}
	for _, field := range whoamiFields {
		value, ok := WhoamiFieldValue(m[field.key])
		if !ok {
			continue
		}
		lines = append(lines, struct {
			label string
			value string
		}{field.label, FormatWhoamiField(field.key, value)})
	}
	if len(lines) == 0 {
		PrintMuted("(no active account)")
		return
	}

	labelWidth := 0
	for _, line := range lines {
		if w := lipgloss.Width(line.label + ":"); w > labelWidth {
			labelWidth = w
		}
	}

	lipgloss.Println(TitleStyle.Render("Active account"))
	for _, line := range lines {
		lipgloss.Printf("  %s %s\n",
			WhoamiKeyStyle.Width(labelWidth).Align(lipgloss.Right).Render(line.label+":"),
			WhoamiValStyle.Render(line.value),
		)
	}
}
