package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Skill represents a reusable team capability.
type Skill struct {
	ID           int64      `json:"id"`
	UUID         string     `json:"uuid"`
	Name         string     `json:"name"`
	Category     string     `json:"category"`
	Description  string     `json:"description"`
	Content      string     `json:"content"`
	Enabled      bool       `json:"enabled"`
	UseCount     int        `json:"use_count"`
	Source       string     `json:"source"` // Manual, Team Shared, GitHub, Agent Local
	Version      string     `json:"version"`
	Tags         string     `json:"tags"` // JSON array
	Favorite     bool       `json:"favorite"`
	InputParams  string     `json:"input_params"`
	OutputFormat string     `json:"output_format"`
	GitHubURL    string     `json:"github_url"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    *time.Time `json:"updated_at"`
}

// SkillLog represents a skill invocation record.
type SkillLog struct {
	ID        int64      `json:"id"`
	SkillID   int64      `json:"skill_id"`
	AgentID   *int64     `json:"agent_id"`
	ProjectID *int64     `json:"project_id"`
	Input     string     `json:"input"`
	Output    string     `json:"output"`
	Success   bool       `json:"success"`
	CreatedAt time.Time  `json:"created_at"`
}

// skillCols is the SELECT column list for skills.
const skillCols = `id, uuid, name, category, description, content, enabled, use_count, source, version, tags, favorite, input_params, output_format, github_url, created_at, updated_at`

// scanSkill scans a skill row.
func scanSkill(scanner interface{ Scan(...any) error }) (Skill, error) {
	var s Skill
	var enabled, fav int
	var updatedAt sql.NullTime
	err := scanner.Scan(&s.ID, &s.UUID, &s.Name, &s.Category, &s.Description, &s.Content,
		&enabled, &s.UseCount, &s.Source, &s.Version, &s.Tags, &fav, &s.InputParams, &s.OutputFormat, &s.GitHubURL,
		&s.CreatedAt, &updatedAt)
	if err != nil {
		return s, err
	}
	s.Enabled = enabled == 1
	s.Favorite = fav == 1
	if updatedAt.Valid {
		s.UpdatedAt = &updatedAt.Time
	}
	return s, nil
}

// CreateSkill inserts a new skill.
func (db *DB) CreateSkill(s *Skill) error {
	now := Now()
	if s.Source == "" {
		s.Source = "Manual"
	}
	if s.Version == "" {
		s.Version = "1.0"
	}
	if s.Tags == "" {
		s.Tags = "[]"
	}
	result, err := db.Conn.Exec(
		`INSERT INTO skills (uuid, name, category, description, content, enabled, use_count, source, version, tags, favorite, input_params, output_format, github_url, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.UUID, s.Name, s.Category, s.Description, s.Content, s.Enabled,
		s.Source, s.Version, s.Tags, s.Favorite, s.InputParams, s.OutputFormat, s.GitHubURL, now, now,
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
	return nil
}

// ListSkills returns all skills ordered by category, name.
func (db *DB) ListSkills() ([]Skill, error) {
	rows, err := db.Conn.Query(`SELECT `+skillCols+` FROM skills ORDER BY category, name`)
	if err != nil {
		return nil, fmt.Errorf("query skills: %w", err)
	}
	defer rows.Close()

	var skills []Skill
	for rows.Next() {
		s, err := scanSkill(rows)
		if err != nil {
			return nil, fmt.Errorf("scan skill: %w", err)
		}
		skills = append(skills, s)
	}
	return skills, rows.Err()
}

// GetSkill returns a single skill by ID.
func (db *DB) GetSkill(id int64) (*Skill, error) {
	s, err := scanSkill(db.Conn.QueryRow(`SELECT `+skillCols+` FROM skills WHERE id = ?`, id))
	if err != nil {
		return nil, fmt.Errorf("get skill %d: %w", id, err)
	}
	return &s, nil
}

// UpdateSkill updates an existing skill.
func (db *DB) UpdateSkill(s *Skill) error {
	now := Now()
	result, err := db.Conn.Exec(
		`UPDATE skills SET name=?, category=?, description=?, content=?, enabled=?, source=?, version=?, tags=?, favorite=?, input_params=?, output_format=?, github_url=?, updated_at=?
		 WHERE id=?`,
		s.Name, s.Category, s.Description, s.Content, s.Enabled, s.Source, s.Version, s.Tags, s.Favorite, s.InputParams, s.OutputFormat, s.GitHubURL, now, s.ID,
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

// ToggleSkillFavorite toggles the favorite field.
func (db *DB) ToggleSkillFavorite(id int64) error {
	_, err := db.Conn.Exec(`UPDATE skills SET favorite = 1 - favorite, updated_at=? WHERE id=?`, Now(), id)
	return err
}

// IncrementSkillUseCount increments use_count by 1.
func (db *DB) IncrementSkillUseCount(id int64) error {
	_, err := db.Conn.Exec(`UPDATE skills SET use_count=use_count+1, updated_at=? WHERE id=?`, Now(), id)
	return err
}

// GetSkillsStats returns aggregated skill statistics.
func (db *DB) GetSkillsStats() (map[string]any, error) {
	stats := map[string]any{}

	var total, enabled, totalUses int
	db.Conn.QueryRow(`SELECT COUNT(*) FROM skills`).Scan(&total)
	db.Conn.QueryRow(`SELECT COUNT(*) FROM skills WHERE enabled=1`).Scan(&enabled)
	db.Conn.QueryRow(`SELECT COALESCE(SUM(use_count), 0) FROM skills`).Scan(&totalUses)
	stats["total"] = total
	stats["enabled"] = enabled
	stats["total_uses"] = totalUses

	var mostUsed string
	if err := db.Conn.QueryRow(`SELECT name FROM skills ORDER BY use_count DESC LIMIT 1`).Scan(&mostUsed); err == nil {
		stats["most_used"] = mostUsed
	}

	return stats, nil
}

// --- Skill Logs ---

// CreateSkillLog records a skill invocation.
func (db *DB) CreateSkillLog(log *SkillLog) error {
	now := Now()
	result, err := db.Conn.Exec(
		`INSERT INTO skill_logs (skill_id, agent_id, project_id, input, output, success, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		log.SkillID, log.AgentID, log.ProjectID, log.Input, log.Output, log.Success, now,
	)
	if err != nil {
		return fmt.Errorf("insert skill log: %w", err)
	}
	id, _ := result.LastInsertId()
	log.ID = id
	log.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", now)
	return nil
}

// ListSkillLogs returns recent logs for a skill.
func (db *DB) ListSkillLogs(skillID int64, limit int) ([]SkillLog, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.Conn.Query(
		`SELECT id, skill_id, agent_id, project_id, input, output, success, created_at
		 FROM skill_logs WHERE skill_id = ? ORDER BY id DESC LIMIT ?`, skillID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query skill logs: %w", err)
	}
	defer rows.Close()

	var logs []SkillLog
	for rows.Next() {
		var l SkillLog
		var success int
		if err := rows.Scan(&l.ID, &l.SkillID, &l.AgentID, &l.ProjectID, &l.Input, &l.Output, &success, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan skill log: %w", err)
		}
		l.Success = success == 1
		logs = append(logs, l)
	}
	return logs, rows.Err()
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
		`SELECT `+skillCols+` FROM skills s JOIN role_skills rs ON s.id = rs.skill_id WHERE rs.role_id = ? ORDER BY s.name`, roleID,
	)
	if err != nil {
		return nil, fmt.Errorf("query role skills: %w", err)
	}
	defer rows.Close()

	var skills []Skill
	for rows.Next() {
		s, err := scanSkill(rows)
		if err != nil {
			return nil, fmt.Errorf("scan role skill: %w", err)
		}
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
		`SELECT `+skillCols+` FROM skills s JOIN agent_skills as2 ON s.id = as2.skill_id WHERE as2.agent_id = ? ORDER BY s.name`, agentID,
	)
	if err != nil {
		return nil, fmt.Errorf("query agent skills: %w", err)
	}
	defer rows.Close()

	var skills []Skill
	for rows.Next() {
		s, err := scanSkill(rows)
		if err != nil {
			return nil, fmt.Errorf("scan agent skill: %w", err)
		}
		skills = append(skills, s)
	}
	return skills, rows.Err()
}

// ListAgentSkillsMap returns a map of agent_id -> []Skill for all agents.
func (db *DB) ListAgentSkillsMap() (map[int64][]Skill, error) {
	rows, err := db.Conn.Query(
		`SELECT as2.agent_id, s.id, s.uuid, s.name, s.category, s.description, s.content, s.enabled, s.use_count, s.source, s.version, s.tags, s.favorite, s.input_params, s.output_format, s.github_url, s.created_at, s.updated_at FROM skills s JOIN agent_skills as2 ON s.id = as2.skill_id ORDER BY as2.agent_id, s.name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := map[int64][]Skill{}
	for rows.Next() {
		var agentID int64
		var s Skill
		var enabled, fav int
		var updatedAt sql.NullTime
		if err := rows.Scan(&agentID, &s.ID, &s.UUID, &s.Name, &s.Category, &s.Description, &s.Content,
			&enabled, &s.UseCount, &s.Source, &s.Version, &s.Tags, &fav, &s.InputParams, &s.OutputFormat, &s.GitHubURL,
			&s.CreatedAt, &updatedAt); err != nil {
			return nil, err
		}
		s.Enabled = enabled == 1
		s.Favorite = fav == 1
		if updatedAt.Valid {
			s.UpdatedAt = &updatedAt.Time
		}
		m[agentID] = append(m[agentID], s)
	}
	return m, rows.Err()
}
