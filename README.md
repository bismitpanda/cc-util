# cc-util

Utilities for Claude Code.

Manage saved accounts: save the logged-in account, switch to another, and list, rename, disable, or inspect them. All cc-util data is stored in one SQLite database, `~/.cc-util/cc-util.db`.

> **macOS is not supported yet.** Claude Code stores its OAuth credentials in the macOS Keychain there instead of `~/.claude/.credentials.json`, and cc-util only reads/writes that file. Linux and Windows are unaffected — Claude Code uses `.credentials.json` on both.

## Install

### One-liner (recommended)

Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/bismitpanda/cc-util/main/install.sh | sh
```

Windows (PowerShell):

```powershell
irm https://raw.githubusercontent.com/bismitpanda/cc-util/main/install.ps1 | iex
```

Re-running either script upgrades both binaries (same install path). `ccu` is the same CLI as `cc-util`. Defaults:

| Platform | Install location                                            |
| -------- | ----------------------------------------------------------- |
| Linux    | `~/.local/bin/cc-util` and `~/.local/bin/ccu`               |
| Windows  | `%LOCALAPPDATA%\Programs\cc-util\cc-util.exe` and `ccu.exe` |

Optional env overrides: `CC_UTIL_VERSION` (e.g. `v1.1.1`), `CC_UTIL_INSTALL_DIR`.

### Manual / other

Download a prebuilt binary from the
[latest release](https://github.com/bismitpanda/cc-util/releases/latest)
(Linux and Windows, amd64/arm64).

Or with Go:

```bash
go install github.com/bismitpanda/cc-util@latest
```

Or from a local clone:

```bash
go build -ldflags "-X main.version=$(git rev-parse --short=7 HEAD) -X main.bin=cc-util" -o cc-util .
go build -ldflags "-X main.version=$(git rev-parse --short=7 HEAD) -X main.bin=ccu" -o ccu .
```

`main.bin` is the name printed in help and error messages. It defaults to `cc-util` when unset.

Requires [Claude Code](https://docs.anthropic.com/en/docs/claude-code) (`claude`). Go is only needed for `go install` / local builds.

## Commands

| Command                                | Description                                                                |
| -------------------------------------- | -------------------------------------------------------------------------- |
| `cc-util accounts save [name]`         | Snapshot the currently logged-in account                                   |
| `cc-util accounts sync`                | Update the active account's snapshot from live creds                       |
| `cc-util accounts use [name]`          | Switch to a saved account                                                  |
| `cc-util accounts disable [name]`      | Disable an account (keeps it, skips usage/use)                             |
| `cc-util accounts enable [name]`       | Re-enable a disabled account                                               |
| `cc-util accounts remove [name]`       | Delete a saved account                                                     |
| `cc-util accounts rename [old] [new]`  | Rename a saved account                                                     |
| `cc-util accounts list`                | List saved accounts                                                        |
| `cc-util accounts history`             | Show account switch history                                                |
| `cc-util accounts whoami`              | Show the active account                                                    |
| `cc-util accounts status`              | Show credential validity and expiry                                        |
| `cc-util accounts usage [name]`        | Show rate-limit usage (all accounts, or a named one)                       |
| `cc-util accounts profile [name]`      | Show the account's Chrome profile, or change it with `--set` or `--clear`  |
| `cc-util browser list`                 | List Chrome profiles, extension state, and the accounts that use them      |
| `cc-util browser profile`              | Print the Chrome profile folder that would be used                         |
| `cc-util browser token [--set <token>]` | Save or print the Playwright extension token of a profile                 |
| `cc-util browser mcp -- <command>`     | Run a Playwright MCP server with the profile and token filled in           |
| `cc-util projects [--long]`            | List project folders saved in `~/.claude.json`                             |
| `cc-util memory list [project]`        | List auto memories for the current project, or all with `-a` (`mem`, `ls`) |
| `cc-util memory show <project> <file>` | Print one memory                                                           |
| `cc-util memory issues`                | Index entries with no file, and files missing from the index               |
| `cc-util completion <shell>`           | Print a completion script for Bash, Fish, or Zsh                           |
| `cc-util help`                         | Show help                                                                  |

`acc` and `account` are aliases for `accounts`.

Omit `[name]` in an interactive terminal and you'll get a prompt.

## Shell completion

Choose your shell:

### Zsh

Create a completion directory and add it to `fpath`:

```bash
completion_dir="${ZDOTDIR:-$HOME}/.zfunc"
mkdir -p "$completion_dir"
cc-util completion zsh > "$completion_dir/_cc-util"
```

Add this before `compinit` in `${ZDOTDIR:-$HOME}/.zshrc`:

```zsh
fpath=("${ZDOTDIR:-$HOME}/.zfunc" $fpath)
autoload -Uz compinit
compinit
```

With Oh My Zsh, use its custom completion directory instead; it is already
added to `fpath` and is kept separate from framework updates:

```zsh
completion_dir="${ZSH_CUSTOM:-${ZSH:-$HOME/.oh-my-zsh}/custom}/completions"
mkdir -p "$completion_dir"
cc-util completion zsh > "$completion_dir/_cc-util"
```

Restart Zsh with `exec zsh`.

### Bash

Install
[bash-completion](https://github.com/scop/bash-completion) first, then write
the script to its per-user completion directory:

```bash
completion_dir="${BASH_COMPLETION_USER_DIR:-${XDG_DATA_HOME:-$HOME/.local/share}/bash-completion}/completions"
mkdir -p "$completion_dir"
cc-util completion bash > "$completion_dir/cc-util"
exec bash
```

### Fish

```fish
set -q XDG_CONFIG_HOME; or set XDG_CONFIG_HOME "$HOME/.config"
set completion_dir "$XDG_CONFIG_HOME/fish/completions"
mkdir -p "$completion_dir"
cc-util completion fish > "$completion_dir/cc-util.fish"
exec fish
```

## First-time setup

```bash
claude auth login          # account A
cc-util accounts save personal

claude auth logout
claude auth login          # account B
cc-util accounts save work

cc-util accounts use personal     # switch without another browser login
cc-util accounts use work
```

## How it works

Each save stores `oauthAccount` (from `~/.claude.json`) and `claudeAiOauth` (from `~/.claude/.credentials.json`, or `$CLAUDE_CONFIG_DIR/.credentials.json` if set) as a row in the `accounts` table of `~/.cc-util/cc-util.db`.

`sync` writes the current live credentials back into the active account's snapshot (errors if the active account isn't saved yet).

`use` syncs the outgoing account's snapshot first (when it matches a saved account), then writes the target snapshot into the active Claude Code config files. Each successful switch adds a row to the `switches` table (`ts`, `from`, `to`).

`disable` marks a snapshot as disabled without deleting it — useful when a subscription is paused or expired and API calls return 403. Disabled accounts stay in `list` (marked) but are skipped by `usage`/`use`/`status` until `enable`.

The database is stored with mode `0600`, and `~/.cc-util/` is `0700`. Nothing in it is encrypted.

## Browser

`browser` works with your own Chrome profiles through the [Playwright extension](https://chromewebstore.google.com/detail/mmlmfjhmonkocbjadbfplnigmagldckm), so the logins you already have in those profiles keep working. Each command takes `--profile` (the folder name such as `Profile 1`, the profile's name, or its Google email) and `--account` (a saved account).

Without `--profile`, cc-util uses the Chrome profile of the active account, or of `--account`. Set it with `cc-util accounts profile <name> --set <profile>`, and remove it with `--clear`. If none is set, cc-util picks the Chrome profile signed in with the account's email, and fails if there is none or more than one. `accounts list` shows each account's profile.

cc-util never opens Chrome itself. Install the extension in each profile you want to use; `browser list` shows where it is missing, and `token` and `mcp` stop with the Web Store link when it is.

To skip the extension's approval prompt, click its icon in the profile, copy the token, and save it with `cc-util browser token --set <token>`. Either the token alone or the whole `PLAYWRIGHT_MCP_EXTENSION_TOKEN=...` line works. `token` without `--set` prints the saved one. If you rotate the token in the extension, save the new one.

`mcp` sets `PLAYWRIGHT_MCP_EXTENSION=true`, `PLAYWRIGHT_MCP_PROFILE_DIR_NAME`, and `PLAYWRIGHT_MCP_EXTENSION_TOKEN`, then runs your server command, so you choose the runner and the version:

```bash
claude mcp add browser -- cc-util browser mcp -- bunx @playwright/mcp@latest
claude mcp add browser -- cc-util browser mcp --account work --shell 'npx -y @playwright/mcp@latest'
```

A command after `--` runs directly. With `--shell`, it runs through `sh -c` (`cmd /c` on Windows). On Linux and macOS, cc-util replaces itself with the server process. If no token is saved for the profile, the server runs without one and the extension asks you to allow each connection.
