// Package db handles SQLite initialization and migrations.
package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the SQLite database connection.
type DB struct {
	Conn *sql.DB
}

// New opens a SQLite database and runs migrations.
func New(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Verify connection
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	// Set pragmas
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA foreign_keys=ON",
	}
	for _, p := range pragmas {
		if _, err := conn.Exec(p); err != nil {
			conn.Close()
			return nil, fmt.Errorf("exec %s: %w", p, err)
		}
	}

	db := &DB{Conn: conn}

	// Run migrations
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	log.Printf("[db] SQLite ready: %s (WAL mode)", path)
	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.Conn.Close()
}

// migrate creates all required tables.
func (db *DB) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS agents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			type TEXT NOT NULL,
			endpoint TEXT NOT NULL DEFAULT '',
			binary_path TEXT NOT NULL DEFAULT '',
			args TEXT NOT NULL DEFAULT '{}',
			status TEXT NOT NULL DEFAULT 'offline',
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS api_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider TEXT NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			api_key TEXT NOT NULL,
			endpoint TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS projects (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			git_path TEXT NOT NULL DEFAULT '',
			branch TEXT NOT NULL DEFAULT 'main',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL,
			parent_id INTEGER DEFAULT NULL,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'todo',
			assigned_agent_id INTEGER DEFAULT NULL,
			priority INTEGER NOT NULL DEFAULT 0,
			dag_level INTEGER NOT NULL DEFAULT 0,
			result TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
			FOREIGN KEY (assigned_agent_id) REFERENCES agents(id) ON DELETE SET NULL
		)`,
		`CREATE TABLE IF NOT EXISTS task_edges (
			from_task INTEGER NOT NULL,
			to_task INTEGER NOT NULL,
			PRIMARY KEY (from_task, to_task),
			FOREIGN KEY (from_task) REFERENCES tasks(id) ON DELETE CASCADE,
			FOREIGN KEY (to_task) REFERENCES tasks(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS chat_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			agent_id INTEGER NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS token_usage (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			model TEXT NOT NULL,
			prompt_tokens INTEGER NOT NULL DEFAULT 0,
			completion_tokens INTEGER NOT NULL DEFAULT 0,
			total_tokens INTEGER NOT NULL DEFAULT 0,
			cost REAL NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS memories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			level TEXT NOT NULL DEFAULT 'working',
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			tags TEXT NOT NULL DEFAULT '[]',
			agent_id INTEGER DEFAULT NULL,
			project_id INTEGER DEFAULT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE SET NULL,
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE SET NULL
		)`,
		`CREATE TABLE IF NOT EXISTS roles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			uuid TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL DEFAULT '',
			priority INTEGER NOT NULL DEFAULT 0,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS agent_roles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			agent_id INTEGER NOT NULL,
			role_id INTEGER NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
			FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
			UNIQUE(agent_id)
		)`,
	}

	for _, q := range queries {
		if _, err := db.Conn.Exec(q); err != nil {
			return fmt.Errorf("exec migration: %w\nQuery: %s", err, q)
		}
	}

	// Verify tables exist
	for _, table := range []string{"agents", "api_keys", "projects", "tasks", "task_edges", "chat_messages", "token_usage", "memories", "roles", "agent_roles"} {
		var count int
		err := db.Conn.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
		if err != nil {
			return fmt.Errorf("verify %s table: %w", table, err)
		}
		log.Printf("[db] Table %s: %d rows", table, count)
	}

	log.Printf("[db] Migration complete")
	return nil
}

// Now returns the current timestamp formatted for SQLite.
func Now() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
