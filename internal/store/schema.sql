CREATE TABLE IF NOT EXISTS accounts (
	name TEXT PRIMARY KEY,
	oauth_account TEXT NOT NULL DEFAULT '{}',
	claude_ai_oauth TEXT NOT NULL DEFAULT '{}',
	disabled INTEGER NOT NULL DEFAULT 0,
	account_uuid TEXT GENERATED ALWAYS AS (lower(json_extract(oauth_account, '$.accountUuid'))) VIRTUAL,
	email TEXT GENERATED ALWAYS AS (json_extract(oauth_account, '$.emailAddress')) VIRTUAL
);

CREATE INDEX IF NOT EXISTS accounts_account_uuid ON accounts (account_uuid);

CREATE TABLE IF NOT EXISTS switches (
	id INTEGER PRIMARY KEY,
	ts TEXT NOT NULL,
	from_account TEXT NOT NULL DEFAULT '',
	to_account TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS model_names (
	model_id TEXT PRIMARY KEY,
	display_name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS account_profiles (
	account_uuid TEXT PRIMARY KEY,
	profile_dir TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS browser_tokens (
	user_data_dir TEXT NOT NULL,
	profile_dir TEXT NOT NULL,
	token TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (user_data_dir, profile_dir)
);
