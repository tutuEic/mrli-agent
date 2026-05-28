// Package dispatch routes chat messages to agents and collects responses.
package dispatch

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"mrli-agent/internal/db"
)

// Config holds dispatch configuration.
type Config struct {
	OpenClawToken string // Bearer token for OpenClaw API
}

// Dispatcher sends messages to agents and returns responses.
type Dispatcher struct {
	db     *db.DB
	client *http.Client
	config Config
}

// New creates a new Dispatcher.
func New(database *db.DB) *Dispatcher {
	return &Dispatcher{
		db:     database,
		client: &http.Client{Timeout: 120 * time.Second},
		config: Config{},
	}
}

// NewWithConfig creates a new Dispatcher with configuration.
func NewWithConfig(database *db.DB, cfg Config) *Dispatcher {
	return &Dispatcher{
		db:     database,
		client: &http.Client{Timeout: 120 * time.Second},
		config: cfg,
	}
}

// Send routes a user message to the appropriate agent and returns the response.
func (d *Dispatcher) Send(agent *db.Agent, userMessage string, history []db.ChatMessage) (string, error) {
	switch agent.Type {
	case "openclaw":
		return d.sendOpenAI(agent, userMessage, history)
	case "hermes":
		// Hermes is CLI-based, use hermes -z "prompt"
		return d.sendCLI(agent, "hermes -z "+shellQuote(userMessage))
	case "custom":
		if agent.Endpoint != "" {
			return d.sendOpenAI(agent, userMessage, history)
		}
		if agent.BinaryPath != "" {
			return d.sendCLI(agent, userMessage)
		}
		return "", fmt.Errorf("no endpoint or binary configured for custom agent")
	case "claude-code":
		return d.sendCLI(agent, "claude -p "+shellQuote(userMessage))
	case "codex":
		return d.sendCLI(agent, "codex -q "+shellQuote(userMessage))
	default:
		// Try OpenAI compatible API first, fall back to CLI
		if agent.Endpoint != "" {
			return d.sendOpenAI(agent, userMessage, history)
		}
		return "", fmt.Errorf("unsupported agent type: %s", agent.Type)
	}
}

// sendOpenAI sends a message via OpenAI-compatible API (used by OpenClaw and custom agents).
func (d *Dispatcher) sendOpenAI(agent *db.Agent, userMessage string, history []db.ChatMessage) (string, error) {
	endpoint := strings.TrimRight(agent.Endpoint, "/")
	log.Printf("[dispatch] Sending to %s (type=%s) via OpenAI API at %s", agent.Name, agent.Type, endpoint)

	// Build messages array with history
	var messages []map[string]string
	for _, m := range history {
		messages = append(messages, map[string]string{
			"role":    m.Role,
			"content": m.Content,
		})
	}
	// Add current user message
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": userMessage,
	})

	// Build OpenAI-compatible request
	payload := map[string]any{
		"model":    "openclaw/default",
		"messages": messages,
		"stream":   false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	// Try OpenAI-compatible endpoint
	url := endpoint + "/v1/chat/completions"
	log.Printf("[dispatch] POST %s", url)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Add auth token if configured
	if d.config.OpenClawToken != "" {
		req.Header.Set("Authorization", "Bearer "+d.config.OpenClawToken)
		log.Printf("[dispatch] Added Authorization header")
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	log.Printf("[dispatch] Response %d: %s", resp.StatusCode, truncate(string(respBody), 300))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	// Parse OpenAI response format
	reply := extractOpenAIReply(respBody)
	if reply == "" {
		return "", fmt.Errorf("no reply extracted from response")
	}

	return reply, nil
}

// sendCLI sends a message via CLI command execution.
func (d *Dispatcher) sendCLI(agent *db.Agent, command string) (string, error) {
	log.Printf("[dispatch] CLI: %s", command)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start command: %w", err)
	}

	// Read stdout
	var outBuf strings.Builder
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		outBuf.WriteString(scanner.Text() + "\n")
	}

	// Read stderr
	var errBuf strings.Builder
	errScanner := bufio.NewScanner(stderr)
	for errScanner.Scan() {
		errBuf.WriteString(errScanner.Text() + "\n")
	}

	err := cmd.Wait()
	if err != nil {
		return "", fmt.Errorf("cli error: %w, stderr: %s", err, truncate(errBuf.String(), 200))
	}

	reply := strings.TrimSpace(outBuf.String())
	if reply == "" {
		return fmt.Sprintf("[%s] Command executed but returned no output.", agent.Name), nil
	}
	return reply, nil
}

// extractOpenAIReply extracts reply from OpenAI Chat Completions response format.
func extractOpenAIReply(data []byte) string {
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		log.Printf("[dispatch] JSON parse error: %v", err)
		// Try plain text
		return strings.TrimSpace(string(data))
	}

	// Check for error
	if resp.Error.Message != "" {
		log.Printf("[dispatch] API error: %s", resp.Error.Message)
		return ""
	}

	if len(resp.Choices) > 0 {
		return strings.TrimSpace(resp.Choices[0].Message.Content)
	}

	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
