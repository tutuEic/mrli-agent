package db

import (
	"fmt"
	"time"
)

// TokenUsage represents a token usage record.
type TokenUsage struct {
	ID               int64     `json:"id"`
	Model            string    `json:"model"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	Cost             float64   `json:"cost"`
	CreatedAt        time.Time `json:"created_at"`
}

// TokenUsageSummary holds aggregated token usage stats.
type TokenUsageSummary struct {
	Model            string  `json:"model"`
	TotalCalls       int     `json:"total_calls"`
	TotalPrompt      int     `json:"total_prompt_tokens"`
	TotalCompletion  int     `json:"total_completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	TotalCost        float64 `json:"total_cost"`
}

// RecordTokenUsage inserts a new token usage record.
func (db *DB) RecordTokenUsage(u *TokenUsage) error {
	now := Now()
	result, err := db.Conn.Exec(
		`INSERT INTO token_usage (model, prompt_tokens, completion_tokens, total_tokens, cost, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		u.Model, u.PromptTokens, u.CompletionTokens, u.TotalTokens, u.Cost, now,
	)
	if err != nil {
		return fmt.Errorf("insert token_usage: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}
	u.ID = id
	u.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", now)
	return nil
}

// GetTokenUsageSummary returns aggregated token usage by model.
func (db *DB) GetTokenUsageSummary() ([]TokenUsageSummary, error) {
	rows, err := db.Conn.Query(
		`SELECT model, COUNT(*), SUM(prompt_tokens), SUM(completion_tokens), SUM(total_tokens), SUM(cost)
		 FROM token_usage GROUP BY model ORDER BY SUM(cost) DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query token_usage summary: %w", err)
	}
	defer rows.Close()

	var summaries []TokenUsageSummary
	for rows.Next() {
		var s TokenUsageSummary
		if err := rows.Scan(&s.Model, &s.TotalCalls, &s.TotalPrompt, &s.TotalCompletion, &s.TotalTokens, &s.TotalCost); err != nil {
			return nil, fmt.Errorf("scan token_usage summary: %w", err)
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

// GetTokenUsageTotal returns the total cost across all models.
func (db *DB) GetTokenUsageTotal() (float64, int, error) {
	var totalCost float64
	var totalTokens int
	err := db.Conn.QueryRow(
		`SELECT COALESCE(SUM(cost), 0), COALESCE(SUM(total_tokens), 0) FROM token_usage`,
	).Scan(&totalCost, &totalTokens)
	if err != nil {
		return 0, 0, fmt.Errorf("query token_usage total: %w", err)
	}
	return totalCost, totalTokens, nil
}

// ListTokenUsage returns recent token usage records.
func (db *DB) ListTokenUsage(limit int) ([]TokenUsage, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := db.Conn.Query(
		`SELECT id, model, prompt_tokens, completion_tokens, total_tokens, cost, created_at
		 FROM token_usage ORDER BY id DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query token_usage: %w", err)
	}
	defer rows.Close()

	var records []TokenUsage
	for rows.Next() {
		var u TokenUsage
		if err := rows.Scan(&u.ID, &u.Model, &u.PromptTokens, &u.CompletionTokens, &u.TotalTokens, &u.Cost, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan token_usage: %w", err)
		}
		records = append(records, u)
	}
	return records, rows.Err()
}
