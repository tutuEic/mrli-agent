package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Agent represents a configured AI agent.
type Agent struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	Type         string     `json:"type"`         // hermes, claude-code, codex, openclaw, custom
	Endpoint     string     `json:"endpoint"`     // API endpoint URL
	BinaryPath   string     `json:"binary_path"`  // CLI binary path
	Args         string     `json:"args"`         // JSON-encoded extra args
	Status       string     `json:"status"`       // online, offline, busy, starting, stopping, recovering, unhealthy
	Enabled      bool       `json:"enabled"`
	LastSeen     *time.Time `json:"last_seen"`
	HealthInfo   string     `json:"health_info"`
	RecoverCount int        `json:"recover_count"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// CreateAgent inserts a new agent and returns its ID.
func (db *DB) CreateAgent(a *Agent) error {
	now := Now()
	result, err := db.Conn.Exec(
		`INSERT INTO agents (name, type, endpoint, binary_path, args, status, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Name, a.Type, a.Endpoint, a.BinaryPath, a.Args, "offline", a.Enabled, now, now,
	)
	if err != nil {
		return fmt.Errorf("insert agent: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}
	a.ID = id
	a.Status = "offline"
	a.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", now)
	a.UpdatedAt = a.CreatedAt
	return nil
}

// ListAgents returns all agents ordered by ID.
func (db *DB) ListAgents() ([]Agent, error) {
	rows, err := db.Conn.Query(
		`SELECT id, name, type, endpoint, binary_path, args, status, enabled,
		        last_seen, health_info, recover_count, created_at, updated_at
		 FROM agents ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("query agents: %w", err)
	}
	defer rows.Close()

	var agents []Agent
	for rows.Next() {
		var a Agent
		var enabled int
		var lastSeen sql.NullTime
		if err := rows.Scan(&a.ID, &a.Name, &a.Type, &a.Endpoint, &a.BinaryPath, &a.Args, &a.Status, &enabled,
			&lastSeen, &a.HealthInfo, &a.RecoverCount, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		a.Enabled = enabled == 1
		if lastSeen.Valid {
			a.LastSeen = &lastSeen.Time
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

// GetAgent returns a single agent by ID.
func (db *DB) GetAgent(id int64) (*Agent, error) {
	var a Agent
	var enabled int
	var lastSeen sql.NullTime
	err := db.Conn.QueryRow(
		`SELECT id, name, type, endpoint, binary_path, args, status, enabled,
		        last_seen, health_info, recover_count, created_at, updated_at
		 FROM agents WHERE id = ?`, id,
	).Scan(&a.ID, &a.Name, &a.Type, &a.Endpoint, &a.BinaryPath, &a.Args, &a.Status, &enabled,
		&lastSeen, &a.HealthInfo, &a.RecoverCount, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get agent %d: %w", id, err)
	}
	a.Enabled = enabled == 1
	if lastSeen.Valid {
		a.LastSeen = &lastSeen.Time
	}
	return &a, nil
}

// UpdateAgent updates an existing agent by ID.
func (db *DB) UpdateAgent(a *Agent) error {
	now := Now()
	result, err := db.Conn.Exec(
		`UPDATE agents SET name=?, type=?, endpoint=?, binary_path=?, args=?, enabled=?, updated_at=?
		 WHERE id=?`,
		a.Name, a.Type, a.Endpoint, a.BinaryPath, a.Args, a.Enabled, now, a.ID,
	)
	if err != nil {
		return fmt.Errorf("update agent %d: %w", a.ID, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("agent %d not found", a.ID)
	}
	a.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", now)
	return nil
}

// UpdateAgentStatus updates only the status field.
func (db *DB) UpdateAgentStatus(id int64, status string) error {
	result, err := db.Conn.Exec(
		`UPDATE agents SET status=?, updated_at=? WHERE id=?`,
		status, Now(), id,
	)
	if err != nil {
		return fmt.Errorf("update agent status: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("agent %d not found", id)
	}
	return nil
}

// DeleteAgent removes an agent by ID.
func (db *DB) DeleteAgent(id int64) error {
	result, err := db.Conn.Exec(`DELETE FROM agents WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete agent %d: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("agent %d not found", id)
	}
	return nil
}

// UpdateAgentLastSeen sets the last_seen timestamp to now.
func (db *DB) UpdateAgentLastSeen(id int64) error {
	_, err := db.Conn.Exec(`UPDATE agents SET last_seen=?, updated_at=? WHERE id=?`, Now(), Now(), id)
	return err
}

// UpdateAgentHealth sets the health_info field.
func (db *DB) UpdateAgentHealth(id int64, info string) error {
	_, err := db.Conn.Exec(`UPDATE agents SET health_info=?, updated_at=? WHERE id=?`, info, Now(), id)
	return err
}

// IncrementRecoverCount increments recover_count by 1.
func (db *DB) IncrementRecoverCount(id int64) error {
	_, err := db.Conn.Exec(`UPDATE agents SET recover_count=recover_count+1, updated_at=? WHERE id=?`, Now(), id)
	return err
}
