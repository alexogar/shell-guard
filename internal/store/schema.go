package store

const schemaSQL = `
CREATE TABLE IF NOT EXISTS sessions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,
	shell_path TEXT NOT NULL,
	shell_pid INTEGER NOT NULL,
	workspace_root TEXT NOT NULL,
	current_cwd TEXT NOT NULL,
	status TEXT NOT NULL,
	hostname TEXT NOT NULL,
	username TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS commands (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id INTEGER NOT NULL,
	raw_command TEXT NOT NULL,
	normalized_command TEXT NOT NULL,
	command_family TEXT NOT NULL,
	cwd TEXT NOT NULL,
	repo_root TEXT NOT NULL,
	git_branch TEXT NOT NULL,
	git_dirty INTEGER NOT NULL DEFAULT 0,
	started_at TIMESTAMP NOT NULL,
	finished_at TIMESTAMP,
	duration_ms INTEGER NOT NULL DEFAULT 0,
	exit_code INTEGER,
	status TEXT NOT NULL,
	summary_short TEXT NOT NULL DEFAULT '',
	raw_output_id INTEGER,
	redacted_output_id INTEGER,
	FOREIGN KEY(session_id) REFERENCES sessions(id)
);

CREATE TABLE IF NOT EXISTS outputs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	command_id INTEGER NOT NULL,
	kind TEXT NOT NULL,
	storage_type TEXT NOT NULL,
	body TEXT NOT NULL DEFAULT '',
	path TEXT NOT NULL DEFAULT '',
	size_bytes INTEGER NOT NULL,
	created_at TIMESTAMP NOT NULL,
	FOREIGN KEY(command_id) REFERENCES commands(id)
);

CREATE TABLE IF NOT EXISTS state_snapshots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id INTEGER NOT NULL UNIQUE,
	current_cwd TEXT NOT NULL,
	repo_root TEXT NOT NULL,
	git_branch TEXT NOT NULL,
	git_dirty INTEGER NOT NULL DEFAULT 0,
	last_command_id INTEGER,
	last_exit_code INTEGER,
	last_summary_short TEXT NOT NULL DEFAULT '',
	updated_at TIMESTAMP NOT NULL,
	FOREIGN KEY(session_id) REFERENCES sessions(id)
);
`
