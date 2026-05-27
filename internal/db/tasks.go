package db

import (
	"fmt"
	"time"
)

// Task represents a development task in the DAG.
type Task struct {
	ID              int64      `json:"id"`
	ProjectID       int64      `json:"project_id"`
	ParentID        *int64     `json:"parent_id"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	Status          string     `json:"status"` // todo, running, review, done, failed
	AssignedAgentID *int64     `json:"assigned_agent_id"`
	Priority        int        `json:"priority"`
	DAGLevel        int        `json:"dag_level"`
	Result          string     `json:"result"`
	CreatedAt       time.Time  `json:"created_at"`
}

// CreateTask inserts a new task.
func (db *DB) CreateTask(t *Task) error {
	now := Now()
	result, err := db.Conn.Exec(
		`INSERT INTO tasks (project_id, parent_id, title, description, status, assigned_agent_id, priority, dag_level, result, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ProjectID, t.ParentID, t.Title, t.Description, t.Status, t.AssignedAgentID, t.Priority, t.DAGLevel, t.Result, now,
	)
	if err != nil {
		return fmt.Errorf("insert task: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}
	t.ID = id
	t.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", now)
	return nil
}

// ListTasks returns all tasks for a project, ordered by DAG level and priority.
func (db *DB) ListTasks(projectID int64) ([]Task, error) {
	rows, err := db.Conn.Query(
		`SELECT id, project_id, parent_id, title, description, status, assigned_agent_id, priority, dag_level, result, created_at
		 FROM tasks WHERE project_id = ? ORDER BY dag_level, priority DESC, id`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.ParentID, &t.Title, &t.Description, &t.Status, &t.AssignedAgentID, &t.Priority, &t.DAGLevel, &t.Result, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// ListAllTasks returns all tasks across all projects.
func (db *DB) ListAllTasks() ([]Task, error) {
	rows, err := db.Conn.Query(
		`SELECT id, project_id, parent_id, title, description, status, assigned_agent_id, priority, dag_level, result, created_at
		 FROM tasks ORDER BY project_id, dag_level, priority DESC, id`,
	)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.ParentID, &t.Title, &t.Description, &t.Status, &t.AssignedAgentID, &t.Priority, &t.DAGLevel, &t.Result, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// GetTask returns a single task by ID.
func (db *DB) GetTask(id int64) (*Task, error) {
	var t Task
	err := db.Conn.QueryRow(
		`SELECT id, project_id, parent_id, title, description, status, assigned_agent_id, priority, dag_level, result, created_at
		 FROM tasks WHERE id = ?`, id,
	).Scan(&t.ID, &t.ProjectID, &t.ParentID, &t.Title, &t.Description, &t.Status, &t.AssignedAgentID, &t.Priority, &t.DAGLevel, &t.Result, &t.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get task %d: %w", id, err)
	}
	return &t, nil
}

// UpdateTaskStatus updates only the status field.
func (db *DB) UpdateTaskStatus(id int64, status string) error {
	result, err := db.Conn.Exec(`UPDATE tasks SET status=? WHERE id=?`, status, id)
	if err != nil {
		return fmt.Errorf("update task status: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("task %d not found", id)
	}
	return nil
}

// AssignTask assigns an agent to a task.
func (db *DB) AssignTask(taskID, agentID int64) error {
	result, err := db.Conn.Exec(`UPDATE tasks SET assigned_agent_id=?, status='running' WHERE id=?`, agentID, taskID)
	if err != nil {
		return fmt.Errorf("assign task: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("task %d not found", taskID)
	}
	return nil
}

// UpdateTaskResult updates the result field and sets status.
func (db *DB) UpdateTaskResult(id int64, status, result string) error {
	res, err := db.Conn.Exec(`UPDATE tasks SET status=?, result=? WHERE id=?`, status, result, id)
	if err != nil {
		return fmt.Errorf("update task result: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("task %d not found", id)
	}
	return nil
}

// DeleteTask removes a task by ID.
func (db *DB) DeleteTask(id int64) error {
	result, err := db.Conn.Exec(`DELETE FROM tasks WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete task %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("task %d not found", id)
	}
	return nil
}
