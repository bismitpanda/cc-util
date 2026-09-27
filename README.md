# cc-util

Utilities for Claude Code.

Manage saved accounts: save the logged-in account, switch to another, and list, rename, disable, or inspect them. Account data is stored under `~/.cc-util/accounts/`.

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

Each save stores `oauthAccount` (from `~/.claude.json`) and `claudeAiOauth` (from `~/.claude/.credentials.json`, or `$CLAUDE_CONFIG_DIR/.credentials.json` if set) as `~/.cc-util/accounts/<name>.json`.

`sync` writes the current live credentials back into the active account's snapshot (errors if the active account isn't saved yet).

`use` syncs the outgoing account's snapshot first (when it matches a saved account), then writes the target snapshot into the active Claude Code config files. Each successful switch appends a line to `~/.cc-util/switches.jsonl` (`ts`, `from`, `to`).

`disable` marks a snapshot as disabled without deleting it — useful when a subscription is paused or expired and API calls return 403. Disabled accounts stay in `list` (marked) but are skipped by `usage`/`use`/`status` until `enable`.

Account snapshots are stored with mode `0600`; `~/.cc-util/` and `accounts/` are `0700`.
