package store

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bismitpanda/cc-util/internal/paths"
	_ "modernc.org/sqlite"
)

const schemaVersion = 1

//go:embed schema.sql
var schema string

var ErrNotFound = errors.New("not found")

type Store struct {
	db *sql.DB
}

type Account struct {
	Name          string
	OAuthAccount  map[string]any
	ClaudeAiOauth map[string]any
	Disabled      bool
}

type Switch struct {
	TS   string `json:"ts"`
	From string `json:"from"`
	To   string `json:"to"`
}

var (
	openOnce  sync.Once
	openStore *Store
	openErr   error
)

func Default() (*Store, error) {
	openOnce.Do(func() {
		openStore, openErr = Open(paths.DBFile())
	})
	return openStore, openErr
}

func Open(path string) (*Store, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating %s: %w", dir, err)
	}
	f, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("creating %s: %w", path, err)
	}
	f.Close()
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.init(); err != nil {
		db.Close()
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	return s, nil
}

func (s *Store) init() error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	switch {
	case version == schemaVersion:
		return nil
	case version > schemaVersion:
		return fmt.Errorf("the database is version %d, newer than this cc-util supports (%d)", version, schemaVersion)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(schema); err != nil {
		return err
	}
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
		return err
	}
	return tx.Commit()
}

func encodeObject(v any) (string, error) {
	if v == nil {
		return "{}", nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func decodeObject(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" || raw == "null" {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAccount(row rowScanner) (Account, error) {
	var (
		a                 Account
		oauthRaw, credRaw string
		disabled          int
	)
	if err := row.Scan(&a.Name, &oauthRaw, &credRaw, &disabled); err != nil {
		return Account{}, err
	}
	var err error
	if a.OAuthAccount, err = decodeObject(oauthRaw); err != nil {
		return Account{}, fmt.Errorf("account '%s' has invalid oauthAccount: %w", a.Name, err)
	}
	if a.ClaudeAiOauth, err = decodeObject(credRaw); err != nil {
		return Account{}, fmt.Errorf("account '%s' has invalid claudeAiOauth: %w", a.Name, err)
	}
	a.Disabled = disabled != 0
	return a, nil
}

const accountColumns = "name, oauth_account, claude_ai_oauth, disabled"

func (s *Store) Accounts() ([]Account, error) {
	rows, err := s.db.Query("SELECT " + accountColumns + " FROM accounts ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var accounts []Account
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

func (s *Store) Account(name string) (Account, error) {
	a, err := scanAccount(s.db.QueryRow("SELECT "+accountColumns+" FROM accounts WHERE name = ?", name))
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	return a, err
}

func (s *Store) AccountByUUID(accountUUID string) (Account, error) {
	a, err := scanAccount(s.db.QueryRow("SELECT "+accountColumns+" FROM accounts WHERE account_uuid = lower(?) ORDER BY name LIMIT 1", accountUUID))
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	return a, err
}

func (s *Store) SaveAccount(name string, oauthAccount, claudeAiOauth any) error {
	oauthRaw, err := encodeObject(oauthAccount)
	if err != nil {
		return err
	}
	credRaw, err := encodeObject(claudeAiOauth)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO accounts (name, oauth_account, claude_ai_oauth) VALUES (?, ?, ?)
		ON CONFLICT (name) DO UPDATE SET oauth_account = excluded.oauth_account, claude_ai_oauth = excluded.claude_ai_oauth`,
		name, oauthRaw, credRaw)
	return err
}

func (s *Store) SetClaudeAiOauth(name string, claudeAiOauth any) error {
	credRaw, err := encodeObject(claudeAiOauth)
	if err != nil {
		return err
	}
	return expectOne(s.db.Exec("UPDATE accounts SET claude_ai_oauth = ? WHERE name = ?", credRaw, name))
}

func (s *Store) SetDisabled(name string, disabled bool) error {
	value := 0
	if disabled {
		value = 1
	}
	return expectOne(s.db.Exec("UPDATE accounts SET disabled = ? WHERE name = ?", value, name))
}

func (s *Store) RenameAccount(oldName, newName string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRow("SELECT count(*) FROM accounts WHERE name = ?", newName).Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		return fmt.Errorf("an account named '%s' already exists", newName)
	}
	if err := expectOne(tx.Exec("UPDATE accounts SET name = ? WHERE name = ?", newName, oldName)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RemoveAccount(name string) error {
	return expectOne(s.db.Exec("DELETE FROM accounts WHERE name = ?", name))
}

func expectOne(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) AddSwitch(from, to string) error {
	_, err := s.db.Exec("INSERT INTO switches (ts, from_account, to_account) VALUES (?, ?, ?)",
		time.Now().UTC().Format(time.RFC3339), from, to)
	return err
}

func (s *Store) Switches() ([]Switch, error) {
	rows, err := s.db.Query("SELECT ts, from_account, to_account FROM switches ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var switches []Switch
	for rows.Next() {
		var sw Switch
		if err := rows.Scan(&sw.TS, &sw.From, &sw.To); err != nil {
			return nil, err
		}
		switches = append(switches, sw)
	}
	return switches, rows.Err()
}

func (s *Store) ModelNames() (map[string]string, error) {
	rows, err := s.db.Query("SELECT model_id, display_name FROM model_names")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := map[string]string{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		names[id] = name
	}
	return names, rows.Err()
}

func (s *Store) PutModelName(id, name string) error {
	_, err := s.db.Exec(`INSERT INTO model_names (model_id, display_name) VALUES (?, ?)
		ON CONFLICT (model_id) DO UPDATE SET display_name = excluded.display_name`, id, name)
	return err
}

func (s *Store) AccountProfile(accountUUID string) (string, error) {
	var dir string
	err := s.db.QueryRow("SELECT profile_dir FROM account_profiles WHERE account_uuid = lower(?)", accountUUID).Scan(&dir)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return dir, err
}

func (s *Store) AccountProfiles() (map[string]string, error) {
	rows, err := s.db.Query("SELECT account_uuid, profile_dir FROM account_profiles")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	profiles := map[string]string{}
	for rows.Next() {
		var accountUUID, dir string
		if err := rows.Scan(&accountUUID, &dir); err != nil {
			return nil, err
		}
		profiles[accountUUID] = dir
	}
	return profiles, rows.Err()
}

func (s *Store) SetAccountProfile(accountUUID, profileDir string) error {
	_, err := s.db.Exec(`INSERT INTO account_profiles (account_uuid, profile_dir) VALUES (lower(?), ?)
		ON CONFLICT (account_uuid) DO UPDATE SET profile_dir = excluded.profile_dir`, accountUUID, profileDir)
	return err
}

func (s *Store) ClearAccountProfile(accountUUID string) error {
	return expectOne(s.db.Exec("DELETE FROM account_profiles WHERE account_uuid = lower(?)", accountUUID))
}

func (s *Store) Token(userDataDir, profileDir string) (string, error) {
	var token string
	err := s.db.QueryRow("SELECT token FROM browser_tokens WHERE user_data_dir = ? AND profile_dir = ?", userDataDir, profileDir).Scan(&token)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return token, err
}

func (s *Store) PutToken(userDataDir, profileDir, token string) error {
	_, err := s.db.Exec(`INSERT INTO browser_tokens (user_data_dir, profile_dir, token, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (user_data_dir, profile_dir) DO UPDATE SET token = excluded.token, updated_at = excluded.updated_at
		WHERE browser_tokens.token <> excluded.token`,
		userDataDir, profileDir, token, time.Now().UTC().Format(time.RFC3339))
	return err
}
