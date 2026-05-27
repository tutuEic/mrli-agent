package db

import (
	"fmt"
	"time"
)

// APIKey stores an LLM provider credential.
type APIKey struct {
	ID        int64     `json:"id"`
	Provider  string    `json:"provider"` // deepseek, openai, anthropic, custom
	Name      string    `json:"name"`     // user-defined label
	APIKey    string    `json:"api_key"`  // the actual key (masked in responses)
	Endpoint  string    `json:"endpoint"` // custom base URL
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

// MaskKey returns the key with middle characters replaced.
// e.g., sk-abcd1234567890 -> sk-a********890
func (k *APIKey) MaskKey() {
	key := k.APIKey
	if len(key) <= 8 {
		return
	}
	k.APIKey = key[:4] + "********" + key[len(key)-3:]
}

// CreateAPIKey inserts a new API key.
func (db *DB) CreateAPIKey(k *APIKey) error {
	now := Now()
	result, err := db.Conn.Exec(
		`INSERT INTO api_keys (provider, name, api_key, endpoint, enabled, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		k.Provider, k.Name, k.APIKey, k.Endpoint, k.Enabled, now,
	)
	if err != nil {
		return fmt.Errorf("insert api_key: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}
	k.ID = id
	k.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", now)
	return nil
}

// ListAPIKeys returns all API keys with masked keys.
func (db *DB) ListAPIKeys() ([]APIKey, error) {
	rows, err := db.Conn.Query(
		`SELECT id, provider, name, api_key, endpoint, enabled, created_at
		 FROM api_keys ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("query api_keys: %w", err)
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var k APIKey
		var enabled int
		if err := rows.Scan(&k.ID, &k.Provider, &k.Name, &k.APIKey, &k.Endpoint, &enabled, &k.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan api_key: %w", err)
		}
		k.Enabled = enabled == 1
		k.MaskKey() // Mask before returning
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// GetAPIKey returns a single API key by ID (masked).
func (db *DB) GetAPIKey(id int64) (*APIKey, error) {
	var k APIKey
	var enabled int
	err := db.Conn.QueryRow(
		`SELECT id, provider, name, api_key, endpoint, enabled, created_at
		 FROM api_keys WHERE id = ?`, id,
	).Scan(&k.ID, &k.Provider, &k.Name, &k.APIKey, &k.Endpoint, &enabled, &k.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get api_key %d: %w", id, err)
	}
	k.Enabled = enabled == 1
	k.MaskKey()
	return &k, nil
}

// GetAPIKeyRaw returns a single API key by ID (unmasked, for internal use).
func (db *DB) GetAPIKeyRaw(id int64) (*APIKey, error) {
	var k APIKey
	var enabled int
	err := db.Conn.QueryRow(
		`SELECT id, provider, name, api_key, endpoint, enabled, created_at
		 FROM api_keys WHERE id = ?`, id,
	).Scan(&k.ID, &k.Provider, &k.Name, &k.APIKey, &k.Endpoint, &enabled, &k.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get api_key %d: %w", id, err)
	}
	k.Enabled = enabled == 1
	return &k, nil
}

// UpdateAPIKey updates an existing API key.
func (db *DB) UpdateAPIKey(k *APIKey) error {
	result, err := db.Conn.Exec(
		`UPDATE api_keys SET provider=?, name=?, api_key=?, endpoint=?, enabled=?
		 WHERE id=?`,
		k.Provider, k.Name, k.APIKey, k.Endpoint, k.Enabled, k.ID,
	)
	if err != nil {
		return fmt.Errorf("update api_key %d: %w", k.ID, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("api_key %d not found", k.ID)
	}
	return nil
}

// DeleteAPIKey removes an API key by ID.
func (db *DB) DeleteAPIKey(id int64) error {
	result, err := db.Conn.Exec(`DELETE FROM api_keys WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete api_key %d: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("api_key %d not found", id)
	}
	return nil
}

// GetAPIKeyByProvider returns the first enabled API key for a provider.
func (db *DB) GetAPIKeyByProvider(provider string) (*APIKey, error) {
	var k APIKey
	var enabled int
	err := db.Conn.QueryRow(
		`SELECT id, provider, name, api_key, endpoint, enabled, created_at
		 FROM api_keys WHERE provider=? AND enabled=1 LIMIT 1`, provider,
	).Scan(&k.ID, &k.Provider, &k.Name, &k.APIKey, &k.Endpoint, &enabled, &k.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get api_key for %s: %w", provider, err)
	}
	k.Enabled = enabled == 1
	return &k, nil
}
