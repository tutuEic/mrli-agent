package db

import (
	"fmt"
	"time"
)

// Memory represents a memory entry with three levels: working, short, long.
type Memory struct {
	ID        int64     `json:"id"`
	Level     string    `json:"level"` // working, short, long
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Tags      string    `json:"tags"` // JSON array
	AgentID   *int64    `json:"agent_id"`
	ProjectID *int64    `json:"project_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateMemory inserts a new memory entry.
func (db *DB) CreateMemory(m *Memory) error {
	now := Now()
	result, err := db.Conn.Exec(
		`INSERT INTO memories (level, key, value, tags, agent_id, project_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		m.Level, m.Key, m.Value, m.Tags, m.AgentID, m.ProjectID, now, now,
	)
	if err != nil {
		return fmt.Errorf("insert memory: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}
	m.ID = id
	m.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", now)
	m.UpdatedAt = m.CreatedAt
	return nil
}

// ListMemories returns memories filtered by level, agent, or project.
func (db *DB) ListMemories(level string, agentID, projectID int64) ([]Memory, error) {
	query := `SELECT id, level, key, value, tags, agent_id, project_id, created_at, updated_at FROM memories WHERE 1=1`
	args := []any{}

	if level != "" {
		query += " AND level = ?"
		args = append(args, level)
	}
	if agentID > 0 {
		query += " AND agent_id = ?"
		args = append(args, agentID)
	}
	if projectID > 0 {
		query += " AND project_id = ?"
		args = append(args, projectID)
	}

	query += " ORDER BY updated_at DESC"

	rows, err := db.Conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query memories: %w", err)
	}
	defer rows.Close()

	var memories []Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.Level, &m.Key, &m.Value, &m.Tags, &m.AgentID, &m.ProjectID, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		memories = append(memories, m)
	}
	return memories, rows.Err()
}

// SearchMemories searches memories by key or value containing the query.
func (db *DB) SearchMemories(query string) ([]Memory, error) {
	rows, err := db.Conn.Query(
		`SELECT id, level, key, value, tags, agent_id, project_id, created_at, updated_at
		 FROM memories WHERE key LIKE ? OR value LIKE ? ORDER BY updated_at DESC LIMIT 50`,
		"%"+query+"%", "%"+query+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("search memories: %w", err)
	}
	defer rows.Close()

	var memories []Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.Level, &m.Key, &m.Value, &m.Tags, &m.AgentID, &m.ProjectID, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		memories = append(memories, m)
	}
	return memories, rows.Err()
}

// UpdateMemory updates an existing memory entry.
func (db *DB) UpdateMemory(m *Memory) error {
	now := Now()
	result, err := db.Conn.Exec(
		`UPDATE memories SET level=?, key=?, value=?, tags=?, agent_id=?, project_id=?, updated_at=? WHERE id=?`,
		m.Level, m.Key, m.Value, m.Tags, m.AgentID, m.ProjectID, now, m.ID,
	)
	if err != nil {
		return fmt.Errorf("update memory %d: %w", m.ID, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("memory %d not found", m.ID)
	}
	m.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", now)
	return nil
}

// DeleteMemory removes a memory entry by ID.
func (db *DB) DeleteMemory(id int64) error {
	result, err := db.Conn.Exec(`DELETE FROM memories WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete memory %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("memory %d not found", id)
	}
	return nil
}

// GetMemoryStats returns counts by level.
func (db *DB) GetMemoryStats() (map[string]int, error) {
	stats := map[string]int{
		"working": 0,
		"short":   0,
		"long":    0,
		"total":   0,
	}

	rows, err := db.Conn.Query(`SELECT level, COUNT(*) FROM memories GROUP BY level`)
	if err != nil {
		return stats, fmt.Errorf("query memory stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var level string
		var count int
		if err := rows.Scan(&level, &count); err != nil {
			continue
		}
		stats[level] = count
		stats["total"] += count
	}

	return stats, rows.Err()
}
