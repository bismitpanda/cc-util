package accounts

import (
	"errors"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/bismitpanda/cc-util/internal/chrome"
	"github.com/bismitpanda/cc-util/internal/cli"
	"github.com/bismitpanda/cc-util/internal/store"
	"github.com/bismitpanda/cc-util/internal/ui"
	"github.com/spf13/cobra"
)

const (
	profileSourceSet   = "set"
	profileSourceEmail = "email"
	profileSourceNone  = "none"
)

type accountProfileJSON struct {
	Status  string          `json:"status,omitempty"`
	Account string          `json:"account"`
	Source  string          `json:"source,omitempty"`
	Profile *chrome.Profile `json:"profile,omitempty"`
	Missing bool            `json:"missing,omitempty"`
}

func profileCommand() *cobra.Command {
	var set string
	var clear bool
	cmd := &cobra.Command{
		Use:   "profile [name]",
		Short: "Show or set the Chrome profile of a saved account",
		Long: `Show or set the Chrome profile that 'browser' uses for a saved account.

With no name, the active account is used. --set takes the profile's folder name
(such as "Profile 1"), its name, or its Google email. When no profile is set,
'browser' uses the Chrome profile signed in with the account's email.`,
		Example: fmt.Sprintf(`  %[1]s accounts profile
  %[1]s accounts profile work --set "Profile 1"
  %[1]s accounts profile work --clear`, cli.Bin),
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeAnyAccountNames,
		Run: func(_ *cobra.Command, args []string) {
			cmdProfile(optionalName(args), set, clear)
		},
	}
	cmd.Flags().StringVar(&set, "set", "", "Chrome profile to use for the account")
	cmd.Flags().BoolVar(&clear, "clear", false, "Remove the account's Chrome profile, so it is matched by email again")
	_ = cmd.RegisterFlagCompletionFunc("set", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return chrome.Completions(), cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}

func resolveProfileAccount(name string) string {
	if name != "" {
		return requireExistingAccount(name)
	}
	if active, ok := ActiveSavedAccountName(); ok {
		return active
	}
	return resolveAnyAccountName("", cli.Cmd("accounts profile <name>"))
}

func accountUUID(account store.Account) string {
	uuid, _ := stringField(account.OAuthAccount, "accountUuid")
	return strings.ToLower(strings.TrimSpace(uuid))
}

func cmdProfile(name, set string, clear bool) {
	if set != "" && clear {
		ui.Fatalf("pass --set or --clear, not both")
	}
	name = resolveProfileAccount(name)
	account, err := savedAccount(name)
	if err != nil {
		ui.Fatalf("%v", err)
	}
	uuid := accountUUID(account)
	if uuid == "" {
		ui.Fatalf("account '%s' has no Claude account id", name)
	}
	switch {
	case clear:
		if err := db().ClearAccountProfile(uuid); err != nil && !errors.Is(err, store.ErrNotFound) {
			ui.Fatalf("%v", err)
		}
		if cli.JSON {
			ui.PrintJSON(accountProfileJSON{Status: "cleared", Account: name})
			return
		}
		ui.PrintSuccess(fmt.Sprintf("Cleared the Chrome profile of %s.", ui.AccountStyle.Render(name)))
	case set != "":
		profiles, err := chrome.Profiles(chrome.UserDataDir())
		if err != nil {
			ui.Fatalf("%v", err)
		}
		p, err := chrome.MatchProfile(profiles, set)
		if err != nil {
			ui.Fatalf("%v. Run: %s", err, cli.Cmd("browser list"))
		}
		if err := db().SetAccountProfile(uuid, p.Dir); err != nil {
			ui.Fatalf("%v", err)
		}
		if cli.JSON {
			ui.PrintJSON(accountProfileJSON{Status: "set", Account: name, Source: profileSourceSet, Profile: &p})
			return
		}
		ui.PrintSuccess(fmt.Sprintf("Set the Chrome profile of %s to %s.", ui.AccountStyle.Render(name), ui.AccountStyle.Render(p.String())))
	default:
		showProfile(name, account, uuid)
	}
}

func showProfile(name string, account store.Account, uuid string) {
	profiles, profilesErr := chrome.Profiles(chrome.UserDataDir())
	result := accountProfileJSON{Account: name, Source: profileSourceNone}
	email, _ := stringField(account.OAuthAccount, "emailAddress")
	var emailMatches []chrome.Profile

	dir, err := db().AccountProfile(uuid)
	switch {
	case err == nil:
		p := chrome.Profile{Dir: dir, Name: dir}
		result.Missing = profilesErr == nil
		for _, candidate := range profiles {
			if candidate.Dir == dir {
				p = candidate
				result.Missing = false
			}
		}
		result.Source = profileSourceSet
		result.Profile = &p
	case errors.Is(err, store.ErrNotFound):
		emailMatches = chrome.ProfilesByEmail(profiles, email)
		if len(emailMatches) == 1 {
			result.Source = profileSourceEmail
			result.Profile = &emailMatches[0]
		}
	default:
		ui.Fatalf("%v", err)
	}

	if cli.JSON {
		ui.PrintJSON(result)
		return
	}
	switch result.Source {
	case profileSourceSet:
		note := ""
		if result.Missing {
			note = " " + ui.ErrorStyle.Render("(no longer exists in Chrome)")
		}
		lipgloss.Printf("%s %s%s\n", ui.AccountStyle.Render(name+":"), ui.WhoamiValStyle.Render(result.Profile.String()), note)
	case profileSourceEmail:
		lipgloss.Printf("%s %s %s\n", ui.AccountStyle.Render(name+":"), ui.WhoamiValStyle.Render(result.Profile.String()), ui.MutedStyle.Render("(not set, matched by email)"))
	default:
		switch {
		case email == "":
			ui.PrintMuted(fmt.Sprintf("%s has no Chrome profile set and no email to match one by.", name))
		case len(emailMatches) > 1:
			ui.PrintMuted(fmt.Sprintf("%s has no Chrome profile set, and several are signed in as %s (%s).", name, email, chrome.Dirs(emailMatches)))
		default:
			ui.PrintMuted(fmt.Sprintf("%s has no Chrome profile set, and none is signed in as %s.", name, email))
		}
		ui.PrintMuted("Set one with: " + cli.Cmd("accounts profile "+name+" --set <profile>"))
	}
}

func accountProfileLabels() map[string]string {
	set, err := db().AccountProfiles()
	if err != nil || len(set) == 0 {
		return map[string]string{}
	}
	names := map[string]string{}
	if profiles, err := chrome.Profiles(chrome.UserDataDir()); err == nil {
		for _, p := range profiles {
			names[p.Dir] = p.Name
		}
	}
	labels := make(map[string]string, len(set))
	for uuid, dir := range set {
		label := dir
		if name, ok := names[dir]; ok && name != "" && name != dir {
			label = fmt.Sprintf("%s [%s]", name, dir)
		}
		labels[uuid] = label
	}
	return labels
}
