package accounts

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/bismitpanda/cc-util/internal/claude"
	"github.com/bismitpanda/cc-util/internal/cli"
	"github.com/bismitpanda/cc-util/internal/paths"
	"github.com/bismitpanda/cc-util/internal/store"
	"github.com/bismitpanda/cc-util/internal/ui"
	"github.com/spf13/cobra"
)

func readJSONObject(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

func writeJSONObject(path string, v any, perm os.FileMode) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

func orDefault(v any, ok bool, def any) any {
	if !ok || v == nil {
		return def
	}
	return v
}

func EnsureSetup() {
	dir := paths.RootDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		ui.Fatalf("could not create %s: %v", dir, err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		ui.Fatalf("could not chmod %s: %v", dir, err)
	}
	if _, err := store.Default(); err != nil {
		ui.Fatalf("%v", err)
	}

	gf := claude.GlobalFile()
	if _, err := os.Stat(gf); os.IsNotExist(err) {
		if err := os.WriteFile(gf, []byte("{}\n"), 0644); err != nil {
			ui.Fatalf("could not create %s: %v", gf, err)
		}
	}
}

func db() *store.Store {
	s, err := store.Default()
	if err != nil {
		ui.Fatalf("%v", err)
	}
	return s
}

func appendSwitchLog(from, to string) {
	_ = db().AddSwitch(from, to)
}

func savedAccount(name string) (store.Account, error) {
	if err := validateAccountName(name); err != nil {
		return store.Account{}, err
	}
	account, err := db().Account(name)
	if errors.Is(err, store.ErrNotFound) {
		return store.Account{}, fmt.Errorf("no saved account called '%s'", name)
	}
	return account, err
}

func resolveAccountName(name string, usage string) string {
	if name == "" {
		if !isInteractive() {
			ui.Fatalf("Usage: %s", usage)
		}
		return promptSelectEnabledAccount()
	}
	return requireAccountName(name)
}

func resolveAnyAccountName(name string, usage string) string {
	if name == "" {
		if !isInteractive() {
			ui.Fatalf("Usage: %s", usage)
		}
		return promptSelectAnyAccount()
	}
	return requireAccountName(name)
}

func resolveDisabledAccountName(name string, usage string) string {
	if name == "" {
		if !isInteractive() {
			ui.Fatalf("Usage: %s", usage)
		}
		return promptSelectDisabledAccount()
	}
	return requireAccountName(name)
}

func accountExists(name string) bool {
	_, err := savedAccount(name)
	return err == nil
}

func isAccountDisabled(name string) bool {
	account, err := savedAccount(name)
	return err == nil && account.Disabled
}

func setAccountDisabled(name string, disabled bool) error {
	return db().SetDisabled(name, disabled)
}

func requireExistingAccount(name string) string {
	name = requireAccountName(name)
	if !accountExists(name) {
		ui.Fatalf("No saved account called '%s'", name)
	}
	return name
}

func requireEnabledAccount(name string) string {
	name = requireExistingAccount(name)
	if isAccountDisabled(name) {
		ui.Fatalf("Account '%s' is disabled. Run: %s", name, cli.Cmd("accounts enable "+name))
	}
	return name
}

func liveCredentials() (oauth any, claudeAiOauth any, err error) {
	cf := claude.CredFile()
	if _, err := os.Stat(cf); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("no credentials file at %s yet — run 'claude auth login' first", cf)
	}

	global, err := readJSONObject(claude.GlobalFile())
	if err != nil {
		return nil, nil, err
	}
	oauthVal, ok := global["oauthAccount"]
	oauth = orDefault(oauthVal, ok, map[string]any{})

	cred, err := readJSONObject(cf)
	if err != nil {
		return nil, nil, err
	}
	credVal, ok := cred["claudeAiOauth"]
	claudeAiOauth = orDefault(credVal, ok, map[string]any{})
	return oauth, claudeAiOauth, nil
}

func writeAccountSnapshot(name string, oauth, claudeAiOauth any) error {
	return db().SaveAccount(name, oauth, claudeAiOauth)
}

func ActiveSavedAccountName() (string, bool) {
	active, err := activeOAuthAccount()
	if err != nil || active == nil {
		return "", false
	}
	for _, name := range listAccountNames() {
		if isActiveSavedAccount(name, active) {
			return name, true
		}
	}
	return "", false
}

func syncActiveSnapshot() (string, error) {
	name, ok := ActiveSavedAccountName()
	if !ok {
		return "", fmt.Errorf("active account is not saved — run: %s", cli.Cmd("accounts save <name>"))
	}
	oauth, claudeAiOauth, err := liveCredentials()
	if err != nil {
		return "", err
	}
	if err := writeAccountSnapshot(name, oauth, claudeAiOauth); err != nil {
		return "", fmt.Errorf("could not save %s: %w", name, err)
	}
	return name, nil
}

type actionResult struct {
	Status  string `json:"status"`
	Account string `json:"account,omitempty"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
}

func cmdSave(name string) {
	if name == "" {
		if !isInteractive() {
			ui.Fatalf("Usage: %s", cli.Cmd("accounts save <name>"))
		}
		name = promptSaveName()
	} else {
		name = requireAccountName(name)
	}

	oauth, claudeAiOauth, err := liveCredentials()
	if err != nil {
		ui.Fatalf("%v", err)
	}
	if err := writeAccountSnapshot(name, oauth, claudeAiOauth); err != nil {
		ui.Fatalf("could not save %s: %v", name, err)
	}
	if cli.JSON {
		ui.PrintJSON(actionResult{Status: "saved", Account: name})
		return
	}
	ui.PrintSuccess(fmt.Sprintf("Saved the currently logged-in account as %s.", ui.AccountStyle.Render(name)))
}

func cmdSync() {
	name, err := syncActiveSnapshot()
	if err != nil {
		ui.Fatalf("%v", err)
	}
	if cli.JSON {
		ui.PrintJSON(actionResult{Status: "synced", Account: name})
		return
	}
	ui.PrintSuccess(fmt.Sprintf("Synced live credentials into %s.", ui.AccountStyle.Render(name)))
}

func cmdUse(name string) {
	name = resolveAccountName(name, cli.Cmd("accounts use <name>"))
	if !accountExists(name) {
		ui.Fatalf("No saved account called '%s'. Run: %s (while logged into it)", name, cli.Cmd("accounts save "+name))
	}
	if isAccountDisabled(name) {
		ui.Fatalf("Account '%s' is disabled. Run: %s", name, cli.Cmd("accounts enable "+name))
	}

	from, _ := ActiveSavedAccountName()
	if from == name {
		if cli.JSON {
			ui.PrintJSON(actionResult{Status: "unchanged", Account: name})
			return
		}
		ui.PrintMuted(fmt.Sprintf("already using %s", ui.AccountStyle.Render(name)))
		return
	}
	if from != "" {
		if _, err := syncActiveSnapshot(); err != nil {
			ui.Fatalf("%v", err)
		}
	}

	account, err := savedAccount(name)
	if err != nil {
		ui.Fatalf("%v", err)
	}
	oauth := account.OAuthAccount
	cred := account.ClaudeAiOauth

	gf := claude.GlobalFile()
	global, err := readJSONObject(gf)
	if err != nil {
		ui.Fatalf("%v", err)
	}
	global["oauthAccount"] = oauth
	if err := writeJSONObject(gf, global, 0600); err != nil {
		ui.Fatalf("could not write %s: %v", gf, err)
	}

	cf := claude.CredFile()
	var credFileObj map[string]any
	if _, err := os.Stat(cf); err == nil {
		credFileObj, err = readJSONObject(cf)
		if err != nil {
			ui.Fatalf("%v", err)
		}
	} else {
		credFileObj = map[string]any{}
	}
	credFileObj["claudeAiOauth"] = cred
	if err := writeJSONObject(cf, credFileObj, 0600); err != nil {
		ui.Fatalf("could not write %s: %v", cf, err)
	}

	appendSwitchLog(from, name)
	if cli.JSON {
		ui.PrintJSON(actionResult{Status: "switched", Account: name, From: from})
		return
	}
	ui.PrintSuccess(fmt.Sprintf("Switched active account to %s", ui.AccountStyle.Render(name)))
}

func cmdHistory() {
	events, err := db().Switches()
	if err != nil {
		ui.Fatalf("%v", err)
	}
	if cli.JSON {
		out := make([]store.Switch, len(events))
		for i, ev := range events {
			out[len(events)-1-i] = ev
		}
		ui.PrintJSON(out)
		return
	}
	if len(events) == 0 {
		ui.PrintMuted("(no switch history yet)")
		return
	}

	for i := len(events) - 1; i >= 0; i-- {
		ev := events[i]
		from := ev.From
		if from == "" {
			from = "—"
		}
		lipgloss.Printf("%s  %s → %s\n",
			ui.MutedStyle.Render(ev.TS),
			ui.AccountStyle.Render(from),
			ui.AccountStyle.Render(ev.To),
		)
	}
}

func cmdRemove(name string, yes bool) {
	name = resolveAnyAccountName(name, cli.Cmd("accounts remove <name>"))
	if !accountExists(name) {
		ui.Fatalf("No saved account called '%s'", name)
	}
	if !yes {
		if !isInteractive() {
			ui.Fatalf("Removing '%s' requires confirmation; pass --yes/-y or run interactively", name)
		}
		if !promptConfirmRemoval(name) {
			if cli.JSON {
				ui.PrintJSON(actionResult{Status: "cancelled", Account: name})
				return
			}
			ui.PrintMuted("Cancelled.")
			return
		}
	}
	if err := db().RemoveAccount(name); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			ui.Fatalf("No saved account called '%s'", name)
		}
		ui.Fatalf("could not remove %s: %v", name, err)
	}
	if cli.JSON {
		ui.PrintJSON(actionResult{Status: "removed", Account: name})
		return
	}
	ui.PrintSuccess(fmt.Sprintf("Removed saved account %s.", ui.AccountStyle.Render(name)))
}

func removeCommand() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:               "remove [name]",
		Aliases:           []string{"rm"},
		Short:             "Delete a saved account",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeAnyAccountNames,
		Run: func(_ *cobra.Command, args []string) {
			cmdRemove(optionalName(args), yes)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the confirmation prompt")
	return cmd
}

func cmdDisable(name string) {
	name = resolveAccountName(name, cli.Cmd("accounts disable <name>"))
	name = requireEnabledAccount(name)
	if err := setAccountDisabled(name, true); err != nil {
		ui.Fatalf("could not disable %s: %v", name, err)
	}
	if cli.JSON {
		ui.PrintJSON(actionResult{Status: "disabled", Account: name})
		return
	}
	ui.PrintSuccess(fmt.Sprintf("Disabled %s. It stays saved but is skipped by usage/use until re-enabled.", ui.AccountStyle.Render(name)))
}

func cmdEnable(name string) {
	name = resolveDisabledAccountName(name, cli.Cmd("accounts enable <name>"))
	name = requireExistingAccount(name)
	if !isAccountDisabled(name) {
		ui.Fatalf("Account '%s' is not disabled", name)
	}
	if err := setAccountDisabled(name, false); err != nil {
		ui.Fatalf("could not enable %s: %v", name, err)
	}
	if cli.JSON {
		ui.PrintJSON(actionResult{Status: "enabled", Account: name})
		return
	}
	ui.PrintSuccess(fmt.Sprintf("Enabled %s.", ui.AccountStyle.Render(name)))
}

func cmdRename(oldName, newName string) {
	if oldName == "" {
		if !isInteractive() {
			ui.Fatalf("Usage: %s", cli.Cmd("accounts rename <old> <new>"))
		}
		oldName = promptSelectAnyAccount()
	} else {
		oldName = requireAccountName(oldName)
	}
	if newName == "" {
		if !isInteractive() {
			ui.Fatalf("Usage: %s", cli.Cmd("accounts rename <old> <new>"))
		}
		newName = promptAccountName("work")
	} else {
		newName = requireAccountName(newName)
	}
	if oldName == newName {
		ui.Fatalf("old and new names are the same")
	}

	if !accountExists(oldName) {
		ui.Fatalf("No saved account called '%s'", oldName)
	}
	if accountExists(newName) {
		ui.Fatalf("An account named '%s' already exists", newName)
	}
	if err := db().RenameAccount(oldName, newName); err != nil {
		ui.Fatalf("could not rename %s to %s: %v", oldName, newName, err)
	}
	if cli.JSON {
		ui.PrintJSON(actionResult{Status: "renamed", From: oldName, To: newName})
		return
	}
	ui.PrintSuccess(fmt.Sprintf("Renamed %s → %s.", ui.AccountStyle.Render(oldName), ui.AccountStyle.Render(newName)))
}

func oauthFieldPlain(oauth map[string]any, key string) string {
	value, ok := ui.WhoamiFieldValue(oauth[key])
	if !ok {
		return "—"
	}
	return ui.FormatWhoamiField(key, value)
}

func savedOAuthAccount(name string) (map[string]any, bool) {
	account, err := savedAccount(name)
	if err != nil || len(account.OAuthAccount) == 0 {
		return nil, false
	}
	return account.OAuthAccount, true
}

type accountListEntry struct {
	Name         string `json:"name"`
	Active       bool   `json:"active"`
	Disabled     bool   `json:"disabled"`
	Email        string `json:"email,omitempty"`
	Organization string `json:"organization,omitempty"`
	Type         string `json:"type,omitempty"`
	ChromeDir    string `json:"chromeProfile,omitempty"`
}

func accountJSONFields(name string) (email, org, typ string) {
	oauth, ok := savedOAuthAccount(name)
	if !ok {
		return "", "", ""
	}
	if v, ok := ui.WhoamiFieldValue(oauth["emailAddress"]); ok {
		email = ui.FormatWhoamiField("emailAddress", v)
	}
	if v, ok := ui.WhoamiFieldValue(oauth["organizationName"]); ok {
		org = ui.FormatWhoamiField("organizationName", v)
	}
	if v, ok := ui.WhoamiFieldValue(oauth["organizationType"]); ok {
		typ = ui.FormatWhoamiField("organizationType", v)
	}
	return
}

func cmdList() {
	names := listAccountNames()
	if cli.JSON {
		active, _ := activeOAuthAccount()
		dirs, err := db().AccountProfiles()
		if err != nil {
			ui.Fatalf("%v", err)
		}
		out := make([]accountListEntry, len(names))
		for i, name := range names {
			email, org, typ := accountJSONFields(name)
			out[i] = accountListEntry{
				Name:         name,
				Active:       isActiveSavedAccount(name, active),
				Disabled:     isAccountDisabled(name),
				Email:        email,
				Organization: org,
				Type:         typ,
				ChromeDir:    dirs[savedAccountUUID(name)],
			}
		}
		ui.PrintJSON(out)
		return
	}

	ui.InitStyles()
	if len(names) == 0 {
		ui.PrintMuted("(no saved accounts yet)")
		return
	}

	active, _ := activeOAuthAccount()
	chromeProfiles := accountProfileLabels()
	activeRows := make([]bool, len(names))
	disabledRows := make([]bool, len(names))
	rows := make([][]string, len(names))
	for i, name := range names {
		activeRows[i] = isActiveSavedAccount(name, active)
		disabledRows[i] = isAccountDisabled(name)
		account := name
		switch {
		case activeRows[i] && disabledRows[i]:
			account = name + " ● (disabled)"
		case activeRows[i]:
			account = name + " ●"
		case disabledRows[i]:
			account = name + " (disabled)"
		}
		email, org, typ := "—", "—", "—"
		if oauth, ok := savedOAuthAccount(name); ok {
			email = oauthFieldPlain(oauth, "emailAddress")
			org = oauthFieldPlain(oauth, "organizationName")
			typ = oauthFieldPlain(oauth, "organizationType")
		}
		chromeProfile, ok := chromeProfiles[savedAccountUUID(name)]
		if !ok {
			chromeProfile = "—"
		}
		rows[i] = []string{account, email, org, typ, chromeProfile}
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(ui.MutedStyle).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return ui.LabelStyle.Bold(true).Padding(0, 1)
			}
			if col == 0 {
				switch {
				case disabledRows[row]:
					return ui.MutedStyle.Padding(0, 1)
				case activeRows[row]:
					return ui.AccountStyle.Bold(true).Foreground(lipgloss.Color("42")).Padding(0, 1)
				default:
					return ui.AccountStyle.Padding(0, 1)
				}
			}
			if disabledRows[row] {
				return ui.MutedStyle.Padding(0, 1)
			}
			return ui.WhoamiValStyle.Padding(0, 1)
		}).
		Headers("Account", "Email", "Organization", "Type", "Chrome profile").
		Rows(rows...)
	lipgloss.Println(t)
}

func listAccountNames() []string {
	accounts, err := db().Accounts()
	if err != nil {
		ui.Fatalf("%v", err)
	}
	names := make([]string, len(accounts))
	for i, account := range accounts {
		names[i] = account.Name
	}
	return names
}

func savedAccountUUID(name string) string {
	account, err := savedAccount(name)
	if err != nil {
		return ""
	}
	return accountUUID(account)
}

func listEnabledAccountNames() []string {
	var names []string
	for _, name := range listAccountNames() {
		if !isAccountDisabled(name) {
			names = append(names, name)
		}
	}
	return names
}

func listDisabledAccountNames() []string {
	var names []string
	for _, name := range listAccountNames() {
		if isAccountDisabled(name) {
			names = append(names, name)
		}
	}
	return names
}

func activeOAuthAccount() (any, error) {
	global, err := readJSONObject(claude.GlobalFile())
	if err != nil {
		return nil, err
	}
	oauthVal, ok := global["oauthAccount"]
	return orDefault(oauthVal, ok, nil), nil
}

func jsonEqual(a, b any) bool {
	da, err := json.Marshal(a)
	if err != nil {
		return false
	}
	db, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(da) == string(db)
}

func oauthAccountIdentity(oauth any) (string, bool) {
	m, ok := oauth.(map[string]any)
	if !ok || m == nil {
		return "", false
	}
	if uuid, ok := stringField(m, "accountUuid"); ok {
		return "uuid:" + uuid, true
	}
	if email, ok := stringField(m, "emailAddress"); ok {
		return "email:" + strings.ToLower(email), true
	}
	return "", false
}

func sameOAuthAccount(a, b any) bool {
	idA, okA := oauthAccountIdentity(a)
	idB, okB := oauthAccountIdentity(b)
	if okA && okB {
		return idA == idB
	}
	return jsonEqual(a, b)
}

func isActiveSavedAccount(name string, active any) bool {
	if active == nil {
		return false
	}
	saved, ok := savedOAuthAccount(name)
	if !ok {
		return false
	}
	return sameOAuthAccount(active, saved)
}

func optionalName(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

func completeAccountNames(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return listEnabledAccountNames(), cobra.ShellCompDirectiveNoFileComp
}

func completeAnyAccountNames(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return listAccountNames(), cobra.ShellCompDirectiveNoFileComp
}

func completeDisabledAccountNames(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return listDisabledAccountNames(), cobra.ShellCompDirectiveNoFileComp
}

func completeRenameArgs(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) >= 2 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if len(args) == 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return listAccountNames(), cobra.ShellCompDirectiveNoFileComp
}

type whoamiJSON struct {
	Active           bool   `json:"active"`
	Name             string `json:"name,omitempty"`
	Email            string `json:"email,omitempty"`
	Organization     string `json:"organization,omitempty"`
	OrganizationType string `json:"organizationType,omitempty"`
}

func cmdWhoami() {
	oauth, err := activeOAuthAccount()
	if err != nil {
		ui.Fatalf("%v", err)
	}
	if cli.JSON {
		out := whoamiJSON{}
		if m, ok := oauth.(map[string]any); ok {
			if v, ok := ui.WhoamiFieldValue(m["displayName"]); ok {
				out.Name = v
			}
			if v, ok := ui.WhoamiFieldValue(m["emailAddress"]); ok {
				out.Email = v
			}
			if v, ok := ui.WhoamiFieldValue(m["organizationName"]); ok {
				out.Organization = v
			}
			if v, ok := ui.WhoamiFieldValue(m["organizationType"]); ok {
				out.OrganizationType = ui.FormatWhoamiField("organizationType", v)
			}
			out.Active = out.Name != "" || out.Email != "" || out.Organization != "" || out.OrganizationType != ""
		}
		ui.PrintJSON(out)
		return
	}
	ui.PrintWhoami(oauth)
}

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "accounts",
		Aliases: []string{"acc", "account"},
		Short:   "Manage Claude Code accounts",
		Long:    "Manage saved Claude Code accounts: save the logged-in account, switch to another, and list, rename, disable, or inspect them.",
		Example: fmt.Sprintf(`
  # First-time setup for multiple accounts:
  claude auth login               # login with account A
  %s accounts save personal
  claude auth logout
  claude auth login               # login with account B
  %s accounts save work
  %s accounts use personal     # switch without another browser login
  %s accounts use work
`, cli.Bin, cli.Bin, cli.Bin, cli.Bin),
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "save [name]",
			Short: "Snapshot the currently logged-in account",
			Args:  cobra.MaximumNArgs(1),
			Run: func(_ *cobra.Command, args []string) {
				cmdSave(optionalName(args))
			},
		},
		&cobra.Command{
			Use:   "sync",
			Short: "Update the active account's snapshot from live credentials",
			Args:  cobra.NoArgs,
			Run: func(_ *cobra.Command, _ []string) {
				cmdSync()
			},
		},
		&cobra.Command{
			Use:               "use [name]",
			Short:             "Switch to a saved account",
			Args:              cobra.MaximumNArgs(1),
			ValidArgsFunction: completeAccountNames,
			Run: func(_ *cobra.Command, args []string) {
				cmdUse(optionalName(args))
			},
		},
		removeCommand(),
		&cobra.Command{
			Use:               "disable [name]",
			Short:             "Disable a saved account (keeps it, skips API use)",
			Args:              cobra.MaximumNArgs(1),
			ValidArgsFunction: completeAccountNames,
			Run: func(_ *cobra.Command, args []string) {
				cmdDisable(optionalName(args))
			},
		},
		&cobra.Command{
			Use:               "enable [name]",
			Short:             "Re-enable a disabled account",
			Args:              cobra.MaximumNArgs(1),
			ValidArgsFunction: completeDisabledAccountNames,
			Run: func(_ *cobra.Command, args []string) {
				cmdEnable(optionalName(args))
			},
		},
		&cobra.Command{
			Use:               "rename [old] [new]",
			Aliases:           []string{"mv"},
			Short:             "Rename a saved account",
			Args:              cobra.MaximumNArgs(2),
			ValidArgsFunction: completeRenameArgs,
			Run: func(_ *cobra.Command, args []string) {
				oldName, newName := "", ""
				if len(args) > 0 {
					oldName = args[0]
				}
				if len(args) > 1 {
					newName = args[1]
				}
				cmdRename(oldName, newName)
			},
		},
		&cobra.Command{
			Use:     "list",
			Aliases: []string{"ls"},
			Short:   "List saved accounts with details",
			Args:    cobra.NoArgs,
			Run: func(_ *cobra.Command, _ []string) {
				cmdList()
			},
		},
		&cobra.Command{
			Use:   "history",
			Short: "Show account switch history",
			Args:  cobra.NoArgs,
			Run: func(_ *cobra.Command, _ []string) {
				cmdHistory()
			},
		},
		&cobra.Command{
			Use:   "whoami",
			Short: "Show the active account",
			Args:  cobra.NoArgs,
			Run: func(_ *cobra.Command, _ []string) {
				cmdWhoami()
			},
		},
		statusCommand(),
		usageCommand(),
		profileCommand(),
	)
	return cmd
}
