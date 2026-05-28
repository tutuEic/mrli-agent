package db

import (
	"database/sql"
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
	Status          string     `json:"status"` // pending, assigned, running, waiting, review, failed, completed
	AssignedAgentID *int64     `json:"assigned_agent_id"`
	Priority        int        `json:"priority"`
	DAGLevel        int        `json:"dag_level"`
	Result          string     `json:"result"`
	Progress        int        `json:"progress"` // 0-100
	Stage           string     `json:"stage"`    // current stage name
	Deadline        *time.Time `json:"deadline"`
	Tags            string     `json:"tags"`        // JSON array
	Attachments     string     `json:"attachments"` // JSON array
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       *time.Time `json:"updated_at"`
}

// CreateTask inserts a new task.
func (db *DB) CreateTask(t *Task) error {
	now := Now()
	if t.Status == "" {
		t.Status = "pending"
	}
	result, err := db.Conn.Exec(
		`INSERT INTO tasks (project_id, parent_id, title, description, status, assigned_agent_id, priority, dag_level, result, progress, stage, deadline, tags, attachments, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ProjectID, t.ParentID, t.Title, t.Description, t.Status, t.AssignedAgentID, t.Priority, t.DAGLevel, t.Result, t.Progress, t.Stage, t.Deadline, t.Tags, t.Attachments, now, now,
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

// taskColumns is the SELECT column list for tasks.
const taskColumns = `id, project_id, parent_id, title, description, status, assigned_agent_id, priority, dag_level, result, progress, stage, deadline, tags, attachments, created_at, updated_at`

// scanTask scans a task row.
func scanTask(scanner interface{ Scan(...any) error }) (Task, error) {
	var t Task
	var deadline, updatedAt sql.NullTime
	err := scanner.Scan(&t.ID, &t.ProjectID, &t.ParentID, &t.Title, &t.Description, &t.Status,
		&t.AssignedAgentID, &t.Priority, &t.DAGLevel, &t.Result, &t.Progress, &t.Stage,
		&deadline, &t.Tags, &t.Attachments, &t.CreatedAt, &updatedAt)
	if err != nil {
		return t, err
	}
	if deadline.Valid {
		t.Deadline = &deadline.Time
	}
	if updatedAt.Valid {
		t.UpdatedAt = &updatedAt.Time
	}
	return t, nil
}

// ListTasks returns all tasks for a project, ordered by DAG level and priority.
func (db *DB) ListTasks(projectID int64) ([]Task, error) {
	rows, err := db.Conn.Query(
		`SELECT `+taskColumns+` FROM tasks WHERE project_id = ? ORDER BY dag_level, priority DESC, id`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// ListAllTasks returns all tasks across all projects.
func (db *DB) ListAllTasks() ([]Task, error) {
	rows, err := db.Conn.Query(
		`SELECT `+taskColumns+` FROM tasks ORDER BY project_id, dag_level, priority DESC, id`,
	)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// GetTask returns a single task by ID.
func (db *DB) GetTask(id int64) (*Task, error) {
	t, err := scanTask(db.Conn.QueryRow(`SELECT `+taskColumns+` FROM tasks WHERE id = ?`, id))
	if err != nil {
		return nil, fmt.Errorf("get task %d: %w", id, err)
	}
	return &t, nil
}

// UpdateTask updates a task.
func (db *DB) UpdateTask(t *Task) error {
	now := Now()
	result, err := db.Conn.Exec(
		`UPDATE tasks SET title=?, description=?, status=?, assigned_agent_id=?, priority=?, progress=?, stage=?, deadline=?, tags=?, updated_at=?
		 WHERE id=?`,
		t.Title, t.Description, t.Status, t.AssignedAgentID, t.Priority, t.Progress, t.Stage, t.Deadline, t.Tags, now, t.ID,
	)
	if err != nil {
		return fmt.Errorf("update task %d: %w", t.ID, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("task %d not found", t.ID)
	}
	return nil
}

// UpdateTaskStatus updates only the status and progress fields.
func (db *DB) UpdateTaskStatus(id int64, status string, progress int) error {
	result, err := db.Conn.Exec(`UPDATE tasks SET status=?, progress=?, updated_at=? WHERE id=?`, status, progress, Now(), id)
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
	result, err := db.Conn.Exec(`UPDATE tasks SET assigned_agent_id=?, status='assigned', updated_at=? WHERE id=?`, agentID, Now(), taskID)
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
	res, err := db.Conn.Exec(`UPDATE tasks SET status=?, result=?, updated_at=? WHERE id=?`, status, result, Now(), id)
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
