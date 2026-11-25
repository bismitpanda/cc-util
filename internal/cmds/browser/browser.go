package browser

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/bismitpanda/cc-util/internal/chrome"
	"github.com/bismitpanda/cc-util/internal/claude"
	"github.com/bismitpanda/cc-util/internal/cli"
	"github.com/bismitpanda/cc-util/internal/store"
	"github.com/bismitpanda/cc-util/internal/ui"
	"github.com/spf13/cobra"
)

type flags struct {
	profile string
	account string
}

func Command() *cobra.Command {
	f := &flags{}
	cmd := &cobra.Command{
		Use:     "browser",
		Aliases: []string{"br"},
		Short:   "Use Chrome profiles through the Playwright extension",
		Long: fmt.Sprintf(`Use your own Chrome profiles, with their logins, through the Playwright extension.

Without --profile, the Chrome profile of the active account (or --account) is
used. Set it with 'accounts profile <name> --set <profile>'. If none is set,
cc-util picks the Chrome profile signed in with the account's email.

cc-util never opens Chrome itself. Install the extension in each profile you
want to use: %s`, chrome.WebStoreURL),
		Example: fmt.Sprintf(`  %[1]s browser list
  %[1]s browser token --profile work@example.com --set <token>
  %[1]s browser mcp -- bunx @playwright/mcp@latest
  %[1]s browser mcp --account work --shell 'npx @playwright/mcp@latest'`, cli.Bin),
	}
	cmd.PersistentFlags().StringVarP(&f.profile, "profile", "p", "", "Chrome profile: folder name, profile name, or Google email")
	cmd.PersistentFlags().StringVarP(&f.account, "account", "a", "", "Saved account whose Chrome profile to use (default: the active account)")
	_ = cmd.RegisterFlagCompletionFunc("profile", completeProfiles)
	_ = cmd.RegisterFlagCompletionFunc("account", completeAccounts)
	cmd.AddCommand(listCommand(), profileCommand(f), tokenCommand(f), mcpCommand(f))
	return cmd
}

type target struct {
	name  string
	uuid  string
	email string
}

func (t target) label() string {
	switch {
	case t.name != "" && t.email != "":
		return fmt.Sprintf("%s (%s)", t.name, t.email)
	case t.name != "":
		return t.name
	default:
		return t.email
	}
}

func db() *store.Store {
	s, err := store.Default()
	if err != nil {
		ui.Fatalf("%v", err)
	}
	return s
}

func targetAccount(name string) (target, error) {
	name = strings.TrimSpace(name)
	if name != "" {
		account, err := db().Account(name)
		if errors.Is(err, store.ErrNotFound) {
			return target{}, fmt.Errorf("no saved account called '%s'", name)
		}
		if err != nil {
			return target{}, err
		}
		t := targetFromOAuth(account.OAuthAccount)
		t.name = account.Name
		if t.uuid == "" {
			return target{}, fmt.Errorf("account '%s' has no Claude account id", name)
		}
		return t, nil
	}
	oauth, err := claude.ActiveOAuthAccount()
	if err != nil {
		return target{}, err
	}
	t := targetFromOAuth(oauth)
	if t.uuid == "" {
		return target{}, errors.New("no account is logged in to Claude Code. Log in, or pass --account or --profile")
	}
	if account, err := db().AccountByUUID(t.uuid); err == nil {
		t.name = account.Name
	}
	return t, nil
}

func targetFromOAuth(oauth map[string]any) target {
	return target{
		uuid:  strings.ToLower(stringField(oauth, "accountUuid")),
		email: stringField(oauth, "emailAddress"),
	}
}

func stringField(m map[string]any, key string) string {
	value, _ := m[key].(string)
	return strings.TrimSpace(value)
}

func resolveProfile(f *flags) (chrome.Profile, error) {
	profiles, err := chrome.Profiles(chrome.UserDataDir())
	if err != nil {
		return chrome.Profile{}, err
	}
	if strings.TrimSpace(f.profile) != "" {
		return matchProfile(profiles, f.profile)
	}
	t, err := targetAccount(f.account)
	if err != nil {
		return chrome.Profile{}, err
	}
	dir, err := db().AccountProfile(t.uuid)
	switch {
	case err == nil:
		for _, p := range profiles {
			if p.Dir == dir {
				return p, nil
			}
		}
		return chrome.Profile{}, fmt.Errorf("the Chrome profile '%s' set for %s no longer exists. %s", dir, t.label(), setDefaultHint(t))
	case !errors.Is(err, store.ErrNotFound):
		return chrome.Profile{}, err
	}
	matches := chrome.ProfilesByEmail(profiles, t.email)
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return chrome.Profile{}, fmt.Errorf("%s has no Chrome profile set, and no Chrome profile is signed in as %s. %s", t.label(), orDash(t.email), setDefaultHint(t))
	default:
		return chrome.Profile{}, fmt.Errorf("several Chrome profiles are signed in as %s (%s). %s", t.email, chrome.Dirs(matches), setDefaultHint(t))
	}
}

func setDefaultHint(t target) string {
	if t.name != "" {
		return "Set one with: " + cli.Cmd("accounts profile "+t.name+" --set <profile>")
	}
	return fmt.Sprintf("Save the account with '%s', then set its profile with '%s', or pass --profile", cli.Cmd("accounts save <name>"), cli.Cmd("accounts profile <name> --set <profile>"))
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func matchProfile(profiles []chrome.Profile, query string) (chrome.Profile, error) {
	p, err := chrome.MatchProfile(profiles, query)
	if err != nil {
		return chrome.Profile{}, fmt.Errorf("%w. Run: %s", err, cli.Cmd("browser list"))
	}
	return p, nil
}

func checkExtension(udd string, p chrome.Profile) error {
	state, err := chrome.Extension(udd, p.Dir)
	if err != nil {
		return err
	}
	switch state {
	case chrome.ExtensionMissing:
		return fmt.Errorf("the Playwright extension is not installed in Chrome profile %s. Install it in that profile: %s", p, chrome.WebStoreURL)
	case chrome.ExtensionDisabled:
		return fmt.Errorf("the Playwright extension is disabled in Chrome profile %s. Turn it on in chrome://extensions", p)
	}
	return nil
}

type profileEntry struct {
	chrome.Profile
	Extension chrome.ExtensionState `json:"extension"`
	Accounts  []string              `json:"accounts"`
}

func listCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List Chrome profiles and the accounts that use them",
		Args:    cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			cmdList()
		},
	}
}

func cmdList() {
	udd := chrome.UserDataDir()
	profiles, err := chrome.Profiles(udd)
	if err != nil {
		ui.Fatalf("%v", err)
	}
	accountsFor, err := accountsByProfile()
	if err != nil {
		ui.Fatalf("%v", err)
	}
	entries := make([]profileEntry, len(profiles))
	for i, p := range profiles {
		state, err := chrome.Extension(udd, p.Dir)
		if err != nil {
			ui.Fatalf("%v", err)
		}
		entries[i] = profileEntry{Profile: p, Extension: state, Accounts: accountsFor[p.Dir]}
		if entries[i].Accounts == nil {
			entries[i].Accounts = []string{}
		}
	}
	if cli.JSON {
		ui.PrintJSON(entries)
		return
	}
	if len(entries) == 0 {
		ui.PrintMuted("(no Chrome profiles)")
		return
	}
	rows := make([][]string, len(entries))
	for i, e := range entries {
		dir := e.Dir
		if e.LastUsed {
			dir += " ●"
		}
		rows[i] = []string{dir, e.Name, orDash(e.Email), string(e.Extension), orDash(strings.Join(e.Accounts, ", "))}
	}
	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(ui.MutedStyle).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return ui.LabelStyle.Bold(true).Padding(0, 1)
			}
			switch {
			case col == 0:
				return ui.AccountStyle.Padding(0, 1)
			case col == 3 && entries[row].Extension == chrome.ExtensionEnabled:
				return ui.SuccessStyle.Padding(0, 1)
			case col == 3:
				return ui.ErrorStyle.Padding(0, 1)
			default:
				return ui.WhoamiValStyle.Padding(0, 1)
			}
		}).
		Headers("Profile", "Name", "Email", "Extension", "Accounts").
		Rows(rows...)
	lipgloss.Println(t)
}

func accountsByProfile() (map[string][]string, error) {
	profiles, err := db().AccountProfiles()
	if err != nil {
		return nil, err
	}
	accounts, err := db().Accounts()
	if err != nil {
		return nil, err
	}
	names := map[string][]string{}
	for _, account := range accounts {
		uuid := strings.ToLower(stringField(account.OAuthAccount, "accountUuid"))
		names[uuid] = append(names[uuid], account.Name)
	}
	byProfile := map[string][]string{}
	for uuid, dir := range profiles {
		if accountNames, ok := names[uuid]; ok {
			byProfile[dir] = append(byProfile[dir], accountNames...)
		} else {
			byProfile[dir] = append(byProfile[dir], "(unsaved "+uuid[:min(8, len(uuid))]+")")
		}
	}
	return byProfile, nil
}

type profileResult struct {
	chrome.Profile
	UserDataDir string `json:"userDataDir"`
}

func profileCommand(f *flags) *cobra.Command {
	return &cobra.Command{
		Use:   "profile",
		Short: "Print the Chrome profile folder that would be used",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			p, err := resolveProfile(f)
			if err != nil {
				ui.Fatalf("%v", err)
			}
			if cli.JSON {
				ui.PrintJSON(profileResult{Profile: p, UserDataDir: chrome.UserDataDir()})
				return
			}
			fmt.Println(p.Dir)
		},
	}
}

type tokenResult struct {
	Status  string `json:"status,omitempty"`
	Profile string `json:"profile"`
	Token   string `json:"token,omitempty"`
}

func tokenCommand(f *flags) *cobra.Command {
	var set string
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Save or print the Playwright extension token of a Chrome profile",
		Long: fmt.Sprintf(`Save or print the Playwright extension token of a Chrome profile.

Copy the token from the extension's page (click its icon in that profile) and
save it with --set. You can paste the token alone or the whole
%s=... line. Without --set, the saved token is printed.`, envToken),
		Example: fmt.Sprintf(`  %[1]s browser token --set <token>
  %[1]s browser token --profile "Profile 2" --set <token>
  %[1]s browser token`, cli.Bin),
		Args: cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			cmdToken(f, set)
		},
	}
	cmd.Flags().StringVar(&set, "set", "", "Token to save for the profile")
	return cmd
}

func cmdToken(f *flags, set string) {
	p, err := resolveProfile(f)
	if err != nil {
		ui.Fatalf("%v", err)
	}
	udd := chrome.UserDataDir()
	if set != "" {
		token := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(set), envToken+"="))
		if token == "" || strings.ContainsAny(token, " \t\r\n\"'=") {
			ui.Fatalf("that does not look like a Playwright extension token")
		}
		if err := db().PutToken(udd, p.Dir, token); err != nil {
			ui.Fatalf("%v", err)
		}
		if cli.JSON {
			ui.PrintJSON(tokenResult{Status: "saved", Profile: p.Dir})
			return
		}
		ui.PrintSuccess(fmt.Sprintf("Saved the Playwright extension token for %s.", ui.AccountStyle.Render(p.String())))
		return
	}
	token, err := db().Token(udd, p.Dir)
	if errors.Is(err, store.ErrNotFound) {
		ui.Fatalf("no token saved for Chrome profile %s. Copy it from the Playwright extension's page in that profile, then run: %s", p, cli.Cmd(fmt.Sprintf("browser token --profile %q --set <token>", p.Dir)))
	}
	if err != nil {
		ui.Fatalf("%v", err)
	}
	if cli.JSON {
		ui.PrintJSON(tokenResult{Profile: p.Dir, Token: token})
		return
	}
	fmt.Println(token)
}

const (
	envExtension  = "PLAYWRIGHT_MCP_EXTENSION"
	envProfileDir = "PLAYWRIGHT_MCP_PROFILE_DIR_NAME"
	envToken      = "PLAYWRIGHT_MCP_EXTENSION_TOKEN"
)

func mcpCommand(f *flags) *cobra.Command {
	var shell string
	cmd := &cobra.Command{
		Use:   "mcp [--shell <command> | -- <command> [args...]]",
		Short: "Run a Playwright MCP server for a Chrome profile",
		Long: fmt.Sprintf(`Run your Playwright MCP server command with the profile and token filled in.

cc-util sets %s, %s, and %s, then runs
the command. Give it after -- to run it directly, or with --shell to run it
through sh (cmd on Windows).

If no token is saved for the profile, the server runs without one, and the
extension asks you to allow each connection. Save one with 'browser token --set'.`, envExtension, envProfileDir, envToken),
		Example: fmt.Sprintf(`  %[1]s browser mcp -- bunx @playwright/mcp@latest
  %[1]s browser mcp --profile "Profile 2" --shell 'npx -y @playwright/mcp@0.0.70'
  claude mcp add browser -- %[1]s browser mcp -- bunx @playwright/mcp@latest`, cli.Bin),
		Args: func(_ *cobra.Command, args []string) error {
			switch {
			case shell != "" && len(args) > 0:
				return errors.New("give the command with --shell or after --, not both")
			case shell == "" && len(args) == 0:
				return errors.New("give the MCP server command after --, or with --shell")
			}
			return nil
		},
		Run: func(_ *cobra.Command, args []string) {
			cmdMCP(f, args, shell)
		},
	}
	cmd.Flags().StringVar(&shell, "shell", "", "Command to run through the system shell")
	cmd.Flags().SetInterspersed(false)
	return cmd
}

func cmdMCP(f *flags, args []string, shell string) {
	p, err := resolveProfile(f)
	if err != nil {
		ui.Fatalf("%v", err)
	}
	udd := chrome.UserDataDir()
	if err := checkExtension(udd, p); err != nil {
		ui.Fatalf("%v", err)
	}
	token, err := db().Token(udd, p.Dir)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		ui.Fatalf("%v", err)
	}
	vars := map[string]string{envExtension: "true", envProfileDir: p.Dir}
	if token != "" {
		vars[envToken] = token
	}
	if err := run(args, shell, withEnv(os.Environ(), vars)); err != nil {
		ui.Fatalf("%v", err)
	}
}

func withEnv(base []string, vars map[string]string) []string {
	env := make([]string, 0, len(base)+len(vars))
	for _, kv := range base {
		key, _, _ := strings.Cut(kv, "=")
		if _, replaced := vars[key]; replaced || key == envToken {
			continue
		}
		env = append(env, kv)
	}
	for key, value := range vars {
		env = append(env, key+"="+value)
	}
	return env
}

func completeProfiles(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return chrome.Completions(), cobra.ShellCompDirectiveNoFileComp
}

func completeAccounts(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	s, err := store.Default()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	accounts, err := s.Accounts()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	names := make([]string, len(accounts))
	for i, account := range accounts {
		names[i] = account.Name
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
