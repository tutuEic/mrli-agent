// Package dispatch routes chat messages to agents and collects responses.
package dispatch

import (
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

// Dispatcher sends messages to agents and returns responses.
type Dispatcher struct {
	db     *db.DB
	client *http.Client
}

// New creates a new Dispatcher.
func New(database *db.DB) *Dispatcher {
	return &Dispatcher{
		db: database,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Send routes a user message to the appropriate agent and returns the response.
func (d *Dispatcher) Send(agent *db.Agent, userMessage string, history []db.ChatMessage) (string, error) {
	switch agent.Type {
	case "openclaw":
		return d.sendHTTP(agent, userMessage, history)
	case "hermes":
		// Hermes is CLI-based, use hermes -z "prompt"
		return d.sendCLI(agent, "hermes -z "+shellQuote(userMessage))
	case "custom":
		if agent.Endpoint != "" {
			return d.sendHTTP(agent, userMessage, history)
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
		// Try HTTP first, fall back to CLI
		if agent.Endpoint != "" {
			return d.sendHTTP(agent, userMessage, history)
		}
		return "", fmt.Errorf("unsupported agent type: %s", agent.Type)
	}
}

// sendHTTP sends a message via HTTP to the agent's endpoint.
func (d *Dispatcher) sendHTTP(agent *db.Agent, userMessage string, history []db.ChatMessage) (string, error) {
	endpoint := strings.TrimRight(agent.Endpoint, "/")
	log.Printf("[dispatch] Sending to %s (type=%s) at %s", agent.Name, agent.Type, endpoint)

	// Try OpenClaw-compatible chat API: POST /api/chat
	payload := map[string]any{
		"message": userMessage,
		"agent":   agent.Name,
	}

	// Build conversation history for context
	var messages []map[string]string
	for _, m := range history {
		messages = append(messages, map[string]string{
			"role":    m.Role,
			"content": m.Content,
		})
	}
	payload["messages"] = messages

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	// Try multiple endpoint patterns
	endpoints := []string{
		endpoint + "/api/chat",
		endpoint + "/v1/chat/completions",
		endpoint + "/chat",
		endpoint + "/api/agent",
	}

	for _, url := range endpoints {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		log.Printf("[dispatch] Trying endpoint: %s", url)
		resp, err := d.client.Do(req)
		if err != nil {
			log.Printf("[dispatch] %s failed: %v", url, err)
			continue
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}

		log.Printf("[dispatch] %s returned %d: %s", url, resp.StatusCode, truncate(string(respBody), 200))

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			// Try to extract reply from various response formats
			reply := extractReply(respBody)
			if reply != "" {
				return reply, nil
			}
		}
	}

	// If all HTTP attempts fail, try a simple health check to see if agent is reachable
	if err := d.pingAgent(agent); err != nil {
		return "", fmt.Errorf("agent %s unreachable: %w", agent.Name, err)
	}

	return fmt.Sprintf("[%s] I received your message but couldn't generate a response. The agent may not support chat via this endpoint.", agent.Name), nil
}

// sendCLI sends a message via CLI command execution.
func (d *Dispatcher) sendCLI(agent *db.Agent, command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("cli error: %w, output: %s", err, truncate(string(out), 200))
	}

	reply := strings.TrimSpace(string(out))
	if reply == "" {
		return fmt.Sprintf("[%s] Command executed but returned no output.", agent.Name), nil
	}
	return reply, nil
}

// pingAgent checks if an agent is reachable.
func (d *Dispatcher) pingAgent(agent *db.Agent) error {
	if agent.Endpoint == "" {
		return fmt.Errorf("no endpoint")
	}

	endpoint := strings.TrimRight(agent.Endpoint, "/")
	for _, path := range []string{"/health", "/"} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		req, err := http.NewRequestWithContext(ctx, "GET", endpoint+path, nil)
		if err != nil {
			cancel()
			continue
		}
		resp, err := d.client.Do(req)
		cancel()
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode < 500 {
			return nil
		}
	}
	return fmt.Errorf("all health checks failed")
}

// extractReply tries to extract a reply string from various JSON response formats.
func extractReply(data []byte) string {
	// Try OpenAI-style: {"choices":[{"message":{"content":"..."}}]}
	var openai struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &openai); err == nil && len(openai.Choices) > 0 {
		if c := openai.Choices[0].Message.Content; c != "" {
			return c
		}
	}

	// Try {"reply":"..."} or {"message":"..."} or {"content":"..."} or {"response":"..."}
	var simple map[string]any
	if err := json.Unmarshal(data, &simple); err == nil {
		for _, key := range []string{"reply", "message", "content", "response", "text", "output", "result"} {
			if v, ok := simple[key]; ok {
				if s, ok := v.(string); ok && s != "" {
					return s
				}
			}
		}
	}

	// Try plain text
	s := strings.TrimSpace(string(data))
	if s != "" && !strings.HasPrefix(s, "{") && !strings.HasPrefix(s, "[") {
		return s
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
