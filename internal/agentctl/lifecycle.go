// Package agentctl implements agent lifecycle operations: health check, wake, restart, stop.
package agentctl

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"mrli-agent/internal/db"
	"mrli-agent/internal/events"
)

// HealthResult holds the result of an agent health check.
type HealthResult struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message"`
	Latency int64  `json:"latency_ms"`
}

// Checker performs health checks and lifecycle operations on agents.
type Checker struct {
	db  *db.DB
	hub *events.Hub
}

// NewChecker creates a new agent lifecycle checker.
func NewChecker(db *db.DB, hub *events.Hub) *Checker {
	return &Checker{db: db, hub: hub}
}

// Ping checks if an agent is reachable and updates its status.
func (c *Checker) Ping(agent *db.Agent) (*HealthResult, error) {
	start := time.Now()
	result := c.checkHealth(agent)
	result.Latency = time.Since(start).Milliseconds()

	// Update last_seen and health_info
	_ = c.db.UpdateAgentLastSeen(agent.ID)
	info, _ := json.Marshal(map[string]any{
		"healthy": result.Healthy,
		"message": result.Message,
		"latency": result.Latency,
		"checked": time.Now().Format(time.RFC3339),
	})
	_ = c.db.UpdateAgentHealth(agent.ID, string(info))

	if result.Healthy {
		if agent.Status != "online" && agent.Status != "busy" {
			_ = c.db.UpdateAgentStatus(agent.ID, "online")
			c.hub.Broadcast("agent.health", map[string]any{
				"agent_id": agent.ID,
				"name":     agent.Name,
				"status":   "online",
				"message":  result.Message,
			})
		}
	} else {
		if agent.Status == "online" || agent.Status == "busy" {
			_ = c.db.UpdateAgentStatus(agent.ID, "unhealthy")
			c.hub.Broadcast("agent.health", map[string]any{
				"agent_id": agent.ID,
				"name":     agent.Name,
				"status":   "unhealthy",
				"message":  result.Message,
			})
		}
	}

	return result, nil
}

// Wake attempts to start an offline agent.
func (c *Checker) Wake(agent *db.Agent) error {
	_ = c.db.UpdateAgentStatus(agent.ID, "starting")
	c.hub.Broadcast("agent.wakeup", map[string]any{
		"agent_id": agent.ID,
		"name":     agent.Name,
		"status":   "starting",
	})

	var err error
	switch agent.Type {
	case "hermes":
		err = c.wakeHermes(agent)
	case "claude-code":
		err = c.wakeCLI(agent, "claude", "--version")
	case "codex":
		err = c.wakeCLI(agent, "codex", "--version")
	case "openclaw":
		err = c.wakeHTTP(agent)
	default:
		err = c.wakeCustom(agent)
	}

	if err != nil {
		_ = c.db.UpdateAgentStatus(agent.ID, "unhealthy")
		_ = c.db.UpdateAgentHealth(agent.ID, fmt.Sprintf(`{"healthy":false,"message":"wake failed: %v"}`, err))
		_ = c.db.IncrementRecoverCount(agent.ID)
		c.hub.Broadcast("agent.health", map[string]any{
			"agent_id": agent.ID,
			"name":     agent.Name,
			"status":   "unhealthy",
			"message":  fmt.Sprintf("wake failed: %v", err),
		})
		return fmt.Errorf("wake agent %s: %w", agent.Name, err)
	}

	// Verify it's actually running
	time.Sleep(2 * time.Second)
	result := c.checkHealth(agent)
	if result.Healthy {
		_ = c.db.UpdateAgentStatus(agent.ID, "online")
		_ = c.db.UpdateAgentLastSeen(agent.ID)
		_ = c.db.IncrementRecoverCount(agent.ID)
		c.hub.Broadcast("agent.recovered", map[string]any{
			"agent_id": agent.ID,
			"name":     agent.Name,
			"status":   "online",
			"message":  "agent recovered successfully",
		})
		return nil
	}

	_ = c.db.UpdateAgentStatus(agent.ID, "unhealthy")
	return fmt.Errorf("agent %s woke but health check failed: %s", agent.Name, result.Message)
}

// Restart stops and then wakes an agent.
func (c *Checker) Restart(agent *db.Agent) error {
	_ = c.db.UpdateAgentStatus(agent.ID, "stopping")
	c.hub.Broadcast("agent.restart", map[string]any{
		"agent_id": agent.ID,
		"name":     agent.Name,
		"status":   "stopping",
	})

	// Try to stop first
	_ = c.Stop(agent)
	time.Sleep(1 * time.Second)

	return c.Wake(agent)
}

// Stop attempts to stop a running agent.
func (c *Checker) Stop(agent *db.Agent) error {
	_ = c.db.UpdateAgentStatus(agent.ID, "stopping")

	var err error
	switch agent.Type {
	case "hermes", "claude-code", "codex":
		err = c.stopProcess(agent)
	case "openclaw":
		err = c.stopHTTP(agent)
	default:
		err = c.stopProcess(agent)
	}

	if err != nil {
		_ = c.db.UpdateAgentStatus(agent.ID, "unhealthy")
		return fmt.Errorf("stop agent %s: %w", agent.Name, err)
	}

	_ = c.db.UpdateAgentStatus(agent.ID, "offline")
	c.hub.Broadcast("agent.health", map[string]any{
		"agent_id": agent.ID,
		"name":     agent.Name,
		"status":   "offline",
		"message":  "agent stopped",
	})
	return nil
}

// checkHealth performs the actual health check based on agent type.
func (c *Checker) checkHealth(agent *db.Agent) *HealthResult {
	switch agent.Type {
	case "hermes":
		return c.checkHTTP(agent)
	case "claude-code":
		return c.checkCLI("claude", "--version")
	case "codex":
		return c.checkCLI("codex", "--version")
	case "openclaw":
		return c.checkHTTP(agent)
	default:
		return c.checkCustom(agent)
	}
}

// checkHTTP checks an agent via HTTP health endpoint.
func (c *Checker) checkHTTP(agent *db.Agent) *HealthResult {
	endpoint := agent.Endpoint
	if endpoint == "" {
		return &HealthResult{Healthy: false, Message: "no endpoint configured"}
	}

	// Try /health first, then root
	for _, path := range []string{"/health", "/"} {
		url := strings.TrimRight(endpoint, "/") + path
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			continue
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()

		if resp.StatusCode < 500 {
			return &HealthResult{Healthy: true, Message: fmt.Sprintf("HTTP %d at %s", resp.StatusCode, path)}
		}
	}

	return &HealthResult{Healthy: false, Message: "all health endpoints failed"}
}

// checkCLI checks a CLI tool by running it with --version.
func (c *Checker) checkCLI(cmd string, args ...string) *HealthResult {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, cmd, args...).CombinedOutput()
	if err != nil {
		return &HealthResult{Healthy: false, Message: fmt.Sprintf("%s not available: %v", cmd, err)}
	}
	ver := strings.TrimSpace(string(out))
	if len(ver) > 80 {
		ver = ver[:80] + "..."
	}
	return &HealthResult{Healthy: true, Message: ver}
}

// checkCustom checks a custom agent using its endpoint or binary.
func (c *Checker) checkCustom(agent *db.Agent) *HealthResult {
	if agent.Endpoint != "" {
		return c.checkHTTP(agent)
	}
	if agent.BinaryPath != "" {
		return c.checkCLI(agent.BinaryPath, "--version")
	}
	return &HealthResult{Healthy: false, Message: "no endpoint or binary configured"}
}

// --- Wake helpers ---

func (c *Checker) wakeHermes(agent *db.Agent) error {
	if agent.BinaryPath == "" {
		return fmt.Errorf("no binary_path configured")
	}
	// Check if already running
	if c.checkHTTP(agent).Healthy {
		return nil // already running
	}
	return c.startProcess(agent.BinaryPath, agent.Args)
}

func (c *Checker) wakeCLI(agent *db.Agent, cmd string, checkArgs ...string) error {
	// Just verify the CLI is available
	result := c.checkCLI(cmd, checkArgs...)
	if !result.Healthy {
		return fmt.Errorf("%s CLI not available: %s", cmd, result.Message)
	}
	return nil
}

func (c *Checker) wakeHTTP(agent *db.Agent) error {
	if agent.Endpoint == "" {
		return fmt.Errorf("no endpoint configured")
	}
	result := c.checkHTTP(agent)
	if result.Healthy {
		return nil // already running
	}
	return fmt.Errorf("openclaw gateway not reachable at %s", agent.Endpoint)
}

func (c *Checker) wakeCustom(agent *db.Agent) error {
	if agent.BinaryPath != "" {
		return c.startProcess(agent.BinaryPath, agent.Args)
	}
	if agent.Endpoint != "" {
		return c.wakeHTTP(agent)
	}
	return fmt.Errorf("no binary_path or endpoint configured")
}

// startProcess starts a binary in the background.
func (c *Checker) startProcess(binary, argsStr string) error {
	var args []string
	if argsStr != "" && argsStr != "{}" {
		if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
			// Try space-split
			args = strings.Fields(argsStr)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", binary, err)
	}

	log.Printf("[agentctl] Started %s (pid %d)", binary, cmd.Process.Pid)
	return nil
}

// stopProcess kills a process by name.
func (c *Checker) stopProcess(agent *db.Agent) error {
	if agent.BinaryPath == "" {
		return nil
	}
	name := agent.BinaryPath
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	if idx := strings.LastIndex(name, "\\"); idx >= 0 {
		name = name[idx+1:]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try graceful kill first
	cmd := exec.CommandContext(ctx, "pkill", "-f", name)
	_ = cmd.Run()
	return nil
}

// stopHTTP sends a stop signal to an HTTP agent.
func (c *Checker) stopHTTP(agent *db.Agent) error {
	if agent.Endpoint == "" {
		return nil
	}
	url := strings.TrimRight(agent.Endpoint, "/") + "/shutdown"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil // process may already be gone
	}
	resp.Body.Close()
	return nil
}
