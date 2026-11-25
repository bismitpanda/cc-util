package chrome

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProfiles(t *testing.T) {
	udd := t.TempDir()
	writeFile(t, filepath.Join(udd, "Local State"), `{"profile": {"last_used": "Profile 10", "info_cache": {
		"Profile 10": {"name": "Ten", "user_name": "ten@example.com"},
		"Profile 2": {"name": "Two", "user_name": ""},
		"Default": {"name": "Main", "user_name": "main@example.com"}
	}}}`)
	profiles, err := Profiles(udd)
	if err != nil {
		t.Fatal(err)
	}
	want := []Profile{
		{Dir: "Default", Name: "Main", Email: "main@example.com"},
		{Dir: "Profile 2", Name: "Two"},
		{Dir: "Profile 10", Name: "Ten", Email: "ten@example.com", LastUsed: true},
	}
	if len(profiles) != len(want) {
		t.Fatalf("got %+v", profiles)
	}
	for i := range want {
		if profiles[i] != want[i] {
			t.Fatalf("profile %d: got %+v, want %+v", i, profiles[i], want[i])
		}
	}
}

func TestExtensionState(t *testing.T) {
	udd := t.TempDir()
	writeFile(t, filepath.Join(udd, "Default", "Secure Preferences"), `{"extensions": {"settings": {"`+ExtensionID+`": {"disable_reasons": [1]}}}}`)
	writeFile(t, filepath.Join(udd, "Profile 1", "Preferences"), `{"extensions": {"settings": {"`+ExtensionID+`": {"disable_reasons": 0}}}}`)
	writeFile(t, filepath.Join(udd, "Profile 2", "Preferences"), `{"extensions": {"settings": {}}}`)
	for dir, want := range map[string]ExtensionState{
		"Default":   ExtensionDisabled,
		"Profile 1": ExtensionEnabled,
		"Profile 2": ExtensionMissing,
		"Profile 3": ExtensionMissing,
	} {
		got, err := Extension(udd, dir)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%s: got %s, want %s", dir, got, want)
		}
	}
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}
