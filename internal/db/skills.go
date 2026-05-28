package db

import (
	"fmt"
	"time"
)

// Skill represents a reusable team capability.
type Skill struct {
	ID          int64     `json:"id"`
	UUID        string    `json:"uuid"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	Content     string    `json:"content"`
	Enabled     bool      `json:"enabled"`
	UseCount    int       `json:"use_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// RoleSkill represents a role-skill binding.
type RoleSkill struct {
	ID        int64     `json:"id"`
	RoleID    int64     `json:"role_id"`
	SkillID   int64     `json:"skill_id"`
	CreatedAt time.Time `json:"created_at"`
}

// AgentSkill represents an agent-skill binding.
type AgentSkill struct {
	ID        int64     `json:"id"`
	AgentID   int64     `json:"agent_id"`
	SkillID   int64     `json:"skill_id"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateSkill inserts a new skill.
func (db *DB) CreateSkill(s *Skill) error {
	now := Now()
	result, err := db.Conn.Exec(
		`INSERT INTO skills (uuid, name, category, description, content, enabled, use_count, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		s.UUID, s.Name, s.Category, s.Description, s.Content, s.Enabled, now, now,
	)
	if err != nil {
		return fmt.Errorf("insert skill: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}
	s.ID = id
	s.UseCount = 0
	s.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", now)
	s.UpdatedAt = s.CreatedAt
	return nil
}

// ListSkills returns all skills ordered by category, name.
func (db *DB) ListSkills() ([]Skill, error) {
	rows, err := db.Conn.Query(
		`SELECT id, uuid, name, category, description, content, enabled, use_count, created_at, updated_at
		 FROM skills ORDER BY category, name`,
	)
	if err != nil {
		return nil, fmt.Errorf("query skills: %w", err)
	}
	defer rows.Close()

	var skills []Skill
	for rows.Next() {
		var s Skill
		var enabled int
		if err := rows.Scan(&s.ID, &s.UUID, &s.Name, &s.Category, &s.Description, &s.Content,
			&enabled, &s.UseCount, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan skill: %w", err)
		}
		s.Enabled = enabled == 1
		skills = append(skills, s)
	}
	return skills, rows.Err()
}

// GetSkill returns a single skill by ID.
func (db *DB) GetSkill(id int64) (*Skill, error) {
	var s Skill
	var enabled int
	err := db.Conn.QueryRow(
		`SELECT id, uuid, name, category, description, content, enabled, use_count, created_at, updated_at
		 FROM skills WHERE id = ?`, id,
	).Scan(&s.ID, &s.UUID, &s.Name, &s.Category, &s.Description, &s.Content,
		&enabled, &s.UseCount, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get skill %d: %w", id, err)
	}
	s.Enabled = enabled == 1
	return &s, nil
}

// UpdateSkill updates an existing skill.
func (db *DB) UpdateSkill(s *Skill) error {
	now := Now()
	result, err := db.Conn.Exec(
		`UPDATE skills SET name=?, category=?, description=?, content=?, enabled=?, updated_at=?
		 WHERE id=?`,
		s.Name, s.Category, s.Description, s.Content, s.Enabled, now, s.ID,
	)
	if err != nil {
		return fmt.Errorf("update skill %d: %w", s.ID, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("skill %d not found", s.ID)
	}
	s.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", now)
	return nil
}

// DeleteSkill removes a skill by ID.
func (db *DB) DeleteSkill(id int64) error {
	result, err := db.Conn.Exec(`DELETE FROM skills WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete skill %d: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("skill %d not found", id)
	}
	return nil
}

// IncrementSkillUseCount increments use_count by 1.
func (db *DB) IncrementSkillUseCount(id int64) error {
	_, err := db.Conn.Exec(`UPDATE skills SET use_count=use_count+1, updated_at=? WHERE id=?`, Now(), id)
	return err
}

// --- Role-Skill bindings ---

// BindRoleSkill creates a role-skill binding.
func (db *DB) BindRoleSkill(roleID, skillID int64) error {
	_, err := db.Conn.Exec(
		`INSERT OR IGNORE INTO role_skills (role_id, skill_id, created_at) VALUES (?, ?, ?)`,
		roleID, skillID, Now(),
	)
	return err
}

// UnbindRoleSkill removes a role-skill binding.
func (db *DB) UnbindRoleSkill(roleID, skillID int64) error {
	_, err := db.Conn.Exec(`DELETE FROM role_skills WHERE role_id=? AND skill_id=?`, roleID, skillID)
	return err
}

// ListRoleSkills returns all skills bound to a role.
func (db *DB) ListRoleSkills(roleID int64) ([]Skill, error) {
	rows, err := db.Conn.Query(
		`SELECT s.id, s.uuid, s.name, s.category, s.description, s.content, s.enabled, s.use_count, s.created_at, s.updated_at
		 FROM skills s JOIN role_skills rs ON s.id = rs.skill_id
		 WHERE rs.role_id = ? ORDER BY s.name`, roleID,
	)
	if err != nil {
		return nil, fmt.Errorf("query role skills: %w", err)
	}
	defer rows.Close()

	var skills []Skill
	for rows.Next() {
		var s Skill
		var enabled int
		if err := rows.Scan(&s.ID, &s.UUID, &s.Name, &s.Category, &s.Description, &s.Content,
			&enabled, &s.UseCount, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan role skill: %w", err)
		}
		s.Enabled = enabled == 1
		skills = append(skills, s)
	}
	return skills, rows.Err()
}

// --- Agent-Skill bindings ---

// BindAgentSkill creates an agent-skill binding.
func (db *DB) BindAgentSkill(agentID, skillID int64) error {
	_, err := db.Conn.Exec(
		`INSERT OR IGNORE INTO agent_skills (agent_id, skill_id, created_at) VALUES (?, ?, ?)`,
		agentID, skillID, Now(),
	)
	return err
}

// UnbindAgentSkill removes an agent-skill binding.
func (db *DB) UnbindAgentSkill(agentID, skillID int64) error {
	_, err := db.Conn.Exec(`DELETE FROM agent_skills WHERE agent_id=? AND skill_id=?`, agentID, skillID)
	return err
}

// ListAgentSkills returns all skills bound to an agent.
func (db *DB) ListAgentSkills(agentID int64) ([]Skill, error) {
	rows, err := db.Conn.Query(
		`SELECT s.id, s.uuid, s.name, s.category, s.description, s.content, s.enabled, s.use_count, s.created_at, s.updated_at
		 FROM skills s JOIN agent_skills as2 ON s.id = as2.skill_id
		 WHERE as2.agent_id = ? ORDER BY s.name`, agentID,
	)
	if err != nil {
		return nil, fmt.Errorf("query agent skills: %w", err)
	}
	defer rows.Close()

	var skills []Skill
	for rows.Next() {
		var s Skill
		var enabled int
		if err := rows.Scan(&s.ID, &s.UUID, &s.Name, &s.Category, &s.Description, &s.Content,
			&enabled, &s.UseCount, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan agent skill: %w", err)
		}
		s.Enabled = enabled == 1
		skills = append(skills, s)
	}
	return skills, rows.Err()
}

// GetSkillsStats returns aggregated skill statistics.
func (db *DB) GetSkillsStats() (map[string]any, error) {
	stats := map[string]any{}

	var total, enabled, totalUses int
	if err := db.Conn.QueryRow(`SELECT COUNT(*) FROM skills`).Scan(&total); err != nil {
		return nil, err
	}
	stats["total"] = total

	if err := db.Conn.QueryRow(`SELECT COUNT(*) FROM skills WHERE enabled=1`).Scan(&enabled); err != nil {
		return nil, err
	}
	stats["enabled"] = enabled

	if err := db.Conn.QueryRow(`SELECT COALESCE(SUM(use_count), 0) FROM skills`).Scan(&totalUses); err != nil {
		return nil, err
	}
	stats["total_uses"] = totalUses

	var mostUsed string
	if err := db.Conn.QueryRow(`SELECT name FROM skills ORDER BY use_count DESC LIMIT 1`).Scan(&mostUsed); err == nil {
		stats["most_used"] = mostUsed
	}

	return stats, nil
}
