package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.db.Close() })
	return s
}

func TestAccounts(t *testing.T) {
	s := openTest(t)
	oauth := map[string]any{"accountUuid": "AAAA-1", "emailAddress": "a@example.com"}
	if err := s.SaveAccount("work", oauth, map[string]any{"accessToken": "one"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetDisabled("work", true); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveAccount("work", oauth, map[string]any{"accessToken": "two"}); err != nil {
		t.Fatal(err)
	}
	a, err := s.Account("work")
	if err != nil {
		t.Fatal(err)
	}
	if !a.Disabled || a.ClaudeAiOauth["accessToken"] != "two" {
		t.Fatalf("saving again must keep disabled and replace credentials: %+v", a)
	}
	byUUID, err := s.AccountByUUID("aaaa-1")
	if err != nil || byUUID.Name != "work" {
		t.Fatalf("got %+v %v", byUUID, err)
	}
	if err := s.SaveAccount("personal", map[string]any{}, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameAccount("work", "personal"); err == nil {
		t.Fatal("renaming onto an existing account must fail")
	}
	if err := s.RenameAccount("work", "job"); err != nil {
		t.Fatal(err)
	}
	accounts, err := s.Accounts()
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 2 || accounts[0].Name != "job" || accounts[1].Name != "personal" {
		t.Fatalf("got %+v", accounts)
	}
	if err := s.RemoveAccount("job"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Account("job"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
	if err := s.SetClaudeAiOauth("missing", map[string]any{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestSwitchesKeepOrder(t *testing.T) {
	s := openTest(t)
	for _, to := range []string{"a", "b", "c"} {
		if err := s.AddSwitch("x", to); err != nil {
			t.Fatal(err)
		}
	}
	switches, err := s.Switches()
	if err != nil {
		t.Fatal(err)
	}
	if len(switches) != 3 || switches[0].To != "a" || switches[2].To != "c" {
		t.Fatalf("got %+v", switches)
	}
}

func TestAccountProfilesAndTokens(t *testing.T) {
	s := openTest(t)
	if _, err := s.AccountProfile("AAAA"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
	if err := s.SetAccountProfile("AAAA", "Profile 1"); err != nil {
		t.Fatal(err)
	}
	if dir, err := s.AccountProfile("aaaa"); err != nil || dir != "Profile 1" {
		t.Fatalf("got %q %v", dir, err)
	}
	if err := s.PutToken("/udd", "Profile 1", "one"); err != nil {
		t.Fatal(err)
	}
	if err := s.PutToken("/udd", "Profile 1", "two"); err != nil {
		t.Fatal(err)
	}
	if token, err := s.Token("/udd", "Profile 1"); err != nil || token != "two" {
		t.Fatalf("got %q %v", token, err)
	}
}

func TestReopenKeepsSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	first, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.SaveAccount("work", map[string]any{}, nil); err != nil {
		t.Fatal(err)
	}
	first.db.Close()
	second, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.db.Close()
	if _, err := second.Account("work"); err != nil {
		t.Fatal(err)
	}
}
