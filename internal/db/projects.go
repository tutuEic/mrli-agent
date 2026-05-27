package db

import (
	"fmt"
	"time"
)

// Project represents a development project.
type Project struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	GitPath   string    `json:"git_path"`
	Branch    string    `json:"branch"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateProject inserts a new project.
func (db *DB) CreateProject(p *Project) error {
	now := Now()
	result, err := db.Conn.Exec(
		`INSERT INTO projects (name, git_path, branch, created_at) VALUES (?, ?, ?, ?)`,
		p.Name, p.GitPath, p.Branch, now,
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

// ListProjects returns all projects ordered by ID.
func (db *DB) ListProjects() ([]Project, error) {
	rows, err := db.Conn.Query(
		`SELECT id, name, git_path, branch, created_at FROM projects ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("query projects: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.GitPath, &p.Branch, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// GetProject returns a single project by ID.
func (db *DB) GetProject(id int64) (*Project, error) {
	var p Project
	err := db.Conn.QueryRow(
		`SELECT id, name, git_path, branch, created_at FROM projects WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.GitPath, &p.Branch, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get project %d: %w", id, err)
	}
	return &p, nil
}

// UpdateProject updates an existing project.
func (db *DB) UpdateProject(p *Project) error {
	result, err := db.Conn.Exec(
		`UPDATE projects SET name=?, git_path=?, branch=? WHERE id=?`,
		p.Name, p.GitPath, p.Branch, p.ID,
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
