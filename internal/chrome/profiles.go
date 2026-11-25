package chrome

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Profile struct {
	Dir      string `json:"dir"`
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
	LastUsed bool   `json:"lastUsed"`
}

type ExtensionState string

const (
	ExtensionMissing  ExtensionState = "missing"
	ExtensionDisabled ExtensionState = "disabled"
	ExtensionEnabled  ExtensionState = "enabled"
)

func Profiles(userDataDir string) ([]Profile, error) {
	path := filepath.Join(userDataDir, "Local State")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no Chrome profiles found in %s", userDataDir)
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var state struct {
		Profile struct {
			InfoCache map[string]struct {
				Name     string `json:"name"`
				UserName string `json:"user_name"`
			} `json:"info_cache"`
			LastUsed string `json:"last_used"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	lastUsed := state.Profile.LastUsed
	if lastUsed == "" {
		lastUsed = "Default"
	}
	profiles := make([]Profile, 0, len(state.Profile.InfoCache))
	for dir, info := range state.Profile.InfoCache {
		profiles = append(profiles, Profile{
			Dir:      dir,
			Name:     info.Name,
			Email:    strings.TrimSpace(info.UserName),
			LastUsed: dir == lastUsed,
		})
	}
	sort.Slice(profiles, func(i, j int) bool {
		if (profiles[i].Dir == "Default") != (profiles[j].Dir == "Default") {
			return profiles[i].Dir == "Default"
		}
		return profileNumber(profiles[i].Dir) < profileNumber(profiles[j].Dir) ||
			profileNumber(profiles[i].Dir) == profileNumber(profiles[j].Dir) && profiles[i].Dir < profiles[j].Dir
	})
	return profiles, nil
}

func profileNumber(dir string) int {
	var n int
	if _, err := fmt.Sscanf(dir, "Profile %d", &n); err != nil {
		return 1 << 30
	}
	return n
}

func Extension(userDataDir, profileDir string) (ExtensionState, error) {
	for _, name := range []string{"Secure Preferences", "Preferences"} {
		state, found, err := extensionState(filepath.Join(userDataDir, profileDir, name))
		if err != nil {
			return ExtensionMissing, err
		}
		if found {
			return state, nil
		}
	}
	return ExtensionMissing, nil
}

func extensionState(prefsPath string) (ExtensionState, bool, error) {
	data, err := os.ReadFile(prefsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ExtensionMissing, false, nil
		}
		return ExtensionMissing, false, fmt.Errorf("reading %s: %w", prefsPath, err)
	}
	var prefs struct {
		Extensions struct {
			Settings map[string]struct {
				State          *int            `json:"state"`
				DisableReasons json.RawMessage `json:"disable_reasons"`
			} `json:"settings"`
		} `json:"extensions"`
	}
	if err := json.Unmarshal(data, &prefs); err != nil {
		return ExtensionMissing, false, fmt.Errorf("parsing %s: %w", prefsPath, err)
	}
	setting, ok := prefs.Extensions.Settings[ExtensionID]
	if !ok {
		return ExtensionMissing, false, nil
	}
	if setting.State != nil && *setting.State == 0 || hasDisableReason(setting.DisableReasons) {
		return ExtensionDisabled, true, nil
	}
	return ExtensionEnabled, true, nil
}

func hasDisableReason(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var reasons []int
	if err := json.Unmarshal(raw, &reasons); err == nil {
		for _, reason := range reasons {
			if reason != 0 {
				return true
			}
		}
		return false
	}
	var mask int
	if err := json.Unmarshal(raw, &mask); err == nil {
		return mask != 0
	}
	return false
}

func (p Profile) String() string {
	s := p.Name
	if p.Email != "" {
		s += " <" + p.Email + ">"
	}
	return s + " [" + p.Dir + "]"
}

func MatchProfile(profiles []Profile, query string) (Profile, error) {
	query = strings.TrimSpace(query)
	for _, p := range profiles {
		if strings.EqualFold(p.Dir, query) {
			return p, nil
		}
	}
	for _, field := range []func(Profile) string{
		func(p Profile) string { return p.Name },
		func(p Profile) string { return p.Email },
	} {
		var matches []Profile
		for _, p := range profiles {
			if strings.EqualFold(field(p), query) {
				matches = append(matches, p)
			}
		}
		switch len(matches) {
		case 0:
			continue
		case 1:
			return matches[0], nil
		default:
			return Profile{}, fmt.Errorf("'%s' matches several Chrome profiles (%s). Use the folder name instead", query, Dirs(matches))
		}
	}
	return Profile{}, fmt.Errorf("no Chrome profile matches '%s'", query)
}

func ProfilesByEmail(profiles []Profile, email string) []Profile {
	var matches []Profile
	if strings.TrimSpace(email) == "" {
		return nil
	}
	for _, p := range profiles {
		if strings.EqualFold(p.Email, strings.TrimSpace(email)) {
			matches = append(matches, p)
		}
	}
	return matches
}

func Dirs(profiles []Profile) string {
	dirs := make([]string, len(profiles))
	for i, p := range profiles {
		dirs[i] = p.Dir
	}
	return strings.Join(dirs, ", ")
}

func Completions() []string {
	profiles, err := Profiles(UserDataDir())
	if err != nil {
		return nil
	}
	out := make([]string, len(profiles))
	for i, p := range profiles {
		desc := p.Name
		if p.Email != "" {
			desc += " <" + p.Email + ">"
		}
		out[i] = p.Dir + "\t" + desc
	}
	return out
}
