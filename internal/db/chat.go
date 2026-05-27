package db

import (
	"fmt"
	"time"
)

// ChatMessage represents a message in a chat with an agent.
type ChatMessage struct {
	ID        int64     `json:"id"`
	AgentID   int64     `json:"agent_id"`
	Role      string    `json:"role"` // user, assistant, system
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateChatMessage inserts a new chat message.
func (db *DB) CreateChatMessage(m *ChatMessage) error {
	now := Now()
	result, err := db.Conn.Exec(
		`INSERT INTO chat_messages (agent_id, role, content, created_at) VALUES (?, ?, ?, ?)`,
		m.AgentID, m.Role, m.Content, now,
	)
	if err != nil {
		return fmt.Errorf("insert chat message: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}
	m.ID = id
	m.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", now)
	return nil
}

// ListChatMessages returns chat messages for an agent, ordered by time.
func (db *DB) ListChatMessages(agentID int64, limit int) ([]ChatMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.Conn.Query(
		`SELECT id, agent_id, role, content, created_at
		 FROM chat_messages WHERE agent_id = ? ORDER BY id DESC LIMIT ?`,
		agentID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query chat messages: %w", err)
	}
	defer rows.Close()

	var messages []ChatMessage
	for rows.Next() {
		var m ChatMessage
		if err := rows.Scan(&m.ID, &m.AgentID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan chat message: %w", err)
		}
		messages = append(messages, m)
	}

	// Reverse to chronological order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, rows.Err()
}

// DeleteChatMessages removes all messages for an agent.
func (db *DB) DeleteChatMessages(agentID int64) error {
	_, err := db.Conn.Exec(`DELETE FROM chat_messages WHERE agent_id=?`, agentID)
	if err != nil {
		return fmt.Errorf("delete chat messages for agent %d: %w", agentID, err)
	}
	return nil
}
