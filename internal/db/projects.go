package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Project represents a development project.
type Project struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	GitPath     string     `json:"git_path"`
	Branch      string     `json:"branch"`
	Status      string     `json:"status"` // Draft, Active, Running, Blocked, Completed, Archived
	Priority    int        `json:"priority"`
	Owner       string     `json:"owner"`
	Favorite    bool       `json:"favorite"`
	Tags        string     `json:"tags"` // JSON array
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at"`
}

// CreateProject inserts a new project.
func (db *DB) CreateProject(p *Project) error {
	now := Now()
	if p.Status == "" {
		p.Status = "Draft"
	}
	result, err := db.Conn.Exec(
		`INSERT INTO projects (name, description, git_path, branch, status, priority, owner, favorite, tags, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.Description, p.GitPath, p.Branch, p.Status, p.Priority, p.Owner, p.Favorite, p.Tags, now, now,
	)
	if err != nil {
		return fmt.Errorf("insert project: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}
	p.ID = id
	p.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", now)
	return nil
}

// ListProjects returns all projects ordered by priority DESC, ID.
func (db *DB) ListProjects() ([]Project, error) {
	rows, err := db.Conn.Query(
		`SELECT id, name, description, git_path, branch, status, priority, owner, favorite, tags, created_at, updated_at
		 FROM projects ORDER BY priority DESC, id`,
	)
	if err != nil {
		return nil, fmt.Errorf("query projects: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		var fav int
		var updatedAt sql.NullTime
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.GitPath, &p.Branch, &p.Status, &p.Priority, &p.Owner, &fav, &p.Tags, &p.CreatedAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		p.Favorite = fav == 1
		if updatedAt.Valid {
			p.UpdatedAt = &updatedAt.Time
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// GetProject returns a single project by ID.
func (db *DB) GetProject(id int64) (*Project, error) {
	var p Project
	var fav int
	var updatedAt sql.NullTime
	err := db.Conn.QueryRow(
		`SELECT id, name, description, git_path, branch, status, priority, owner, favorite, tags, created_at, updated_at
		 FROM projects WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.GitPath, &p.Branch, &p.Status, &p.Priority, &p.Owner, &fav, &p.Tags, &p.CreatedAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("get project %d: %w", id, err)
	}
	p.Favorite = fav == 1
	if updatedAt.Valid {
		p.UpdatedAt = &updatedAt.Time
	}
	return &p, nil
}

// UpdateProject updates an existing project.
func (db *DB) UpdateProject(p *Project) error {
	now := Now()
	result, err := db.Conn.Exec(
		`UPDATE projects SET name=?, description=?, git_path=?, branch=?, status=?, priority=?, owner=?, favorite=?, tags=?, updated_at=?
		 WHERE id=?`,
		p.Name, p.Description, p.GitPath, p.Branch, p.Status, p.Priority, p.Owner, p.Favorite, p.Tags, now, p.ID,
	)
	if err != nil {
		return fmt.Errorf("update project %d: %w", p.ID, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("project %d not found", p.ID)
	}
	return nil
}

// UpdateProjectStatus updates only the status field.
func (db *DB) UpdateProjectStatus(id int64, status string) error {
	_, err := db.Conn.Exec(`UPDATE projects SET status=?, updated_at=? WHERE id=?`, status, Now(), id)
	return err
}

// ToggleProjectFavorite toggles the favorite field.
func (db *DB) ToggleProjectFavorite(id int64) error {
	_, err := db.Conn.Exec(`UPDATE projects SET favorite = 1 - favorite, updated_at=? WHERE id=?`, Now(), id)
	return err
}

// DeleteProject removes a project by ID.
func (db *DB) DeleteProject(id int64) error {
	result, err := db.Conn.Exec(`DELETE FROM projects WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete project %d: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("project %d not found", id)
	}
	return nil
}

// GetProjectStats returns task statistics for a project.
func (db *DB) GetProjectStats(projectID int64) (map[string]any, error) {
	stats := map[string]any{}
	var total, done, running, failed, pending int
	db.Conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE project_id=?`, projectID).Scan(&total)
	db.Conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE project_id=? AND status='done'`, projectID).Scan(&done)
	db.Conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE project_id=? AND status='running'`, projectID).Scan(&running)
	db.Conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE project_id=? AND status='failed'`, projectID).Scan(&failed)
	db.Conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE project_id=? AND status IN ('todo','pending')`, projectID).Scan(&pending)

	var onlineAgents int
	db.Conn.QueryRow(`SELECT COUNT(*) FROM agents WHERE status IN ('online','busy')`).Scan(&onlineAgents)

	completionRate := 0.0
	if total > 0 {
		completionRate = float64(done) / float64(total) * 100
	}

	stats["total_tasks"] = total
	stats["done_tasks"] = done
	stats["running_tasks"] = running
	stats["failed_tasks"] = failed
	stats["pending_tasks"] = pending
	stats["online_agents"] = onlineAgents
	stats["completion_rate"] = completionRate
	return stats, nil
}

// --- Project-Skill bindings ---

// BindProjectSkill creates a project-skill binding.
func (db *DB) BindProjectSkill(projectID, skillID int64) error {
	_, err := db.Conn.Exec(
		`INSERT OR IGNORE INTO project_skills (project_id, skill_id, enabled, created_at) VALUES (?, ?, 1, ?)`,
		projectID, skillID, Now(),
	)
	return err
}

// UnbindProjectSkill removes a project-skill binding.
func (db *DB) UnbindProjectSkill(projectID, skillID int64) error {
	_, err := db.Conn.Exec(`DELETE FROM project_skills WHERE project_id=? AND skill_id=?`, projectID, skillID)
	return err
}

// ListProjectSkills returns all skills bound to a project.
func (db *DB) ListProjectSkills(projectID int64) ([]Skill, error) {
	rows, err := db.Conn.Query(
		`SELECT s.id, s.uuid, s.name, s.category, s.description, s.content, s.enabled, s.use_count, s.created_at, s.updated_at
		 FROM skills s JOIN project_skills ps ON s.id = ps.skill_id
		 WHERE ps.project_id = ? ORDER BY s.name`, projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("query project skills: %w", err)
	}
	defer rows.Close()

	var skills []Skill
	for rows.Next() {
		var s Skill
		var enabled int
		if err := rows.Scan(&s.ID, &s.UUID, &s.Name, &s.Category, &s.Description, &s.Content,
			&enabled, &s.UseCount, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project skill: %w", err)
		}
		s.Enabled = enabled == 1
		skills = append(skills, s)
	}
	return skills, rows.Err()
}
