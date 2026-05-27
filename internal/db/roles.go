package db

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Role represents a role that agents can take.
type Role struct {
	ID          int64     `json:"id"`
	UUID        string    `json:"uuid"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Priority    int       `json:"priority"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	AgentCount  int       `json:"agent_count,omitempty"`
}

// AgentRole represents the binding between an agent and a role.
type AgentRole struct {
	ID        int64     `json:"id"`
	AgentID   int64     `json:"agent_id"`
	RoleID    int64     `json:"role_id"`
	CreatedAt time.Time `json:"created_at"`
}

// AgentWithRole represents an agent with its role info.
type AgentWithRole struct {
	Agent
	RoleID   *int64  `json:"role_id"`
	RoleName *string `json:"role_name"`
}

func genUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// CreateRole inserts a new role.
func (db *DB) CreateRole(r *Role) error {
	now := Now()
	r.UUID = genUUID()
	result, err := db.Conn.Exec(
		`INSERT INTO roles (uuid, name, description, priority, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.UUID, r.Name, r.Description, r.Priority, r.Enabled, now, now,
	)
	if err != nil {
		return fmt.Errorf("insert role: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}
	r.ID = id
	r.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", now)
	r.UpdatedAt = r.CreatedAt
	return nil
}

// ListRoles returns all roles with agent counts.
func (db *DB) ListRoles() ([]Role, error) {
	rows, err := db.Conn.Query(
		`SELECT r.id, r.uuid, r.name, r.description, r.priority, r.enabled, r.created_at, r.updated_at,
		        COALESCE((SELECT COUNT(*) FROM agent_roles WHERE role_id = r.id), 0) as agent_count
		 FROM roles r ORDER BY r.priority DESC, r.name`,
	)
	if err != nil {
		return nil, fmt.Errorf("query roles: %w", err)
	}
	defer rows.Close()

	var roles []Role
	for rows.Next() {
		var r Role
		var enabled int
		if err := rows.Scan(&r.ID, &r.UUID, &r.Name, &r.Description, &r.Priority, &enabled, &r.CreatedAt, &r.UpdatedAt, &r.AgentCount); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		r.Enabled = enabled == 1
		roles = append(roles, r)
	}
	return roles, rows.Err()
}

// GetRole returns a single role by ID.
func (db *DB) GetRole(id int64) (*Role, error) {
	var r Role
	var enabled int
	err := db.Conn.QueryRow(
		`SELECT id, uuid, name, description, priority, enabled, created_at, updated_at FROM roles WHERE id = ?`, id,
	).Scan(&r.ID, &r.UUID, &r.Name, &r.Description, &r.Priority, &enabled, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get role %d: %w", id, err)
	}
	r.Enabled = enabled == 1
	return &r, nil
}

// UpdateRole updates an existing role.
func (db *DB) UpdateRole(r *Role) error {
	now := Now()
	result, err := db.Conn.Exec(
		`UPDATE roles SET name=?, description=?, priority=?, enabled=?, updated_at=? WHERE id=?`,
		r.Name, r.Description, r.Priority, r.Enabled, now, r.ID,
	)
	if err != nil {
		return fmt.Errorf("update role %d: %w", r.ID, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("role %d not found", r.ID)
	}
	r.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", now)
	return nil
}

// DeleteRole removes a role by ID.
func (db *DB) DeleteRole(id int64) error {
	result, err := db.Conn.Exec(`DELETE FROM roles WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete role %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("role %d not found", id)
	}
	return nil
}

// AssignRole assigns a role to an agent (one agent = one role).
func (db *DB) AssignRole(agentID, roleID int64) error {
	// Verify agent exists
	var count int
	db.Conn.QueryRow("SELECT COUNT(*) FROM agents WHERE id=?", agentID).Scan(&count)
	if count == 0 {
		return fmt.Errorf("agent %d not found", agentID)
	}

	// Verify role exists
	db.Conn.QueryRow("SELECT COUNT(*) FROM roles WHERE id=?", roleID).Scan(&count)
	if count == 0 {
		return fmt.Errorf("role %d not found", roleID)
	}

	// Upsert: insert or replace
	_, err := db.Conn.Exec(
		`INSERT INTO agent_roles (agent_id, role_id, created_at) VALUES (?, ?, ?)
		 ON CONFLICT(agent_id) DO UPDATE SET role_id=?`,
		agentID, roleID, Now(), roleID,
	)
	if err != nil {
		return fmt.Errorf("assign role: %w", err)
	}
	return nil
}

// UnassignRole removes the role assignment for an agent.
func (db *DB) UnassignRole(agentID int64) error {
	_, err := db.Conn.Exec(`DELETE FROM agent_roles WHERE agent_id=?`, agentID)
	if err != nil {
		return fmt.Errorf("unassign role: %w", err)
	}
	return nil
}

// GetAgentRole returns the role for an agent.
func (db *DB) GetAgentRole(agentID int64) (*Role, error) {
	var r Role
	var enabled int
	err := db.Conn.QueryRow(
		`SELECT r.id, r.uuid, r.name, r.description, r.priority, r.enabled, r.created_at, r.updated_at
		 FROM roles r JOIN agent_roles ar ON r.id = ar.role_id WHERE ar.agent_id = ?`, agentID,
	).Scan(&r.ID, &r.UUID, &r.Name, &r.Description, &r.Priority, &enabled, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get agent role: %w", err)
	}
	r.Enabled = enabled == 1
	return &r, nil
}

// ListAgentsWithRole returns all agents with their role info.
func (db *DB) ListAgentsWithRole() ([]AgentWithRole, error) {
	rows, err := db.Conn.Query(
		`SELECT a.id, a.name, a.type, a.endpoint, a.binary_path, a.args, a.status, a.enabled, a.created_at, a.updated_at,
		        ar.role_id, r.name as role_name
		 FROM agents a
		 LEFT JOIN agent_roles ar ON a.id = ar.agent_id
		 LEFT JOIN roles r ON ar.role_id = r.id
		 ORDER BY a.id`,
	)
	if err != nil {
		return nil, fmt.Errorf("query agents with role: %w", err)
	}
	defer rows.Close()

	var agents []AgentWithRole
	for rows.Next() {
		var a AgentWithRole
		var enabled int
		if err := rows.Scan(&a.ID, &a.Name, &a.Type, &a.Endpoint, &a.BinaryPath, &a.Args, &a.Status, &enabled, &a.CreatedAt, &a.UpdatedAt, &a.RoleID, &a.RoleName); err != nil {
			return nil, fmt.Errorf("scan agent with role: %w", err)
		}
		a.Enabled = enabled == 1
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

// GetAgentsByRole returns all agents assigned to a specific role.
func (db *DB) GetAgentsByRole(roleID int64) ([]Agent, error) {
	rows, err := db.Conn.Query(
		`SELECT a.id, a.name, a.type, a.endpoint, a.binary_path, a.args, a.status, a.enabled, a.created_at, a.updated_at
		 FROM agents a JOIN agent_roles ar ON a.id = ar.agent_id WHERE ar.role_id = ?`,
		roleID,
	)
	if err != nil {
		return nil, fmt.Errorf("query agents by role: %w", err)
	}
	defer rows.Close()

	var agents []Agent
	for rows.Next() {
		var a Agent
		var enabled int
		if err := rows.Scan(&a.ID, &a.Name, &a.Type, &a.Endpoint, &a.BinaryPath, &a.Args, &a.Status, &enabled, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		a.Enabled = enabled == 1
		agents = append(agents, a)
	}
	return agents, rows.Err()
}
