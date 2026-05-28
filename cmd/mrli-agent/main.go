package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"mrli-agent/internal/agentctl"
	"mrli-agent/internal/api"
	"mrli-agent/internal/db"
	"mrli-agent/internal/dispatch"
	"mrli-agent/internal/events"
)

func main() {
	port := flag.Int("port", 8080, "server port")
	dbPath := flag.String("db", "mrli-agent.db", "SQLite database path")
	flag.Parse()

	// Open database
	database, err := db.New(*dbPath)
	if err != nil {
		log.Fatalf("[main] Database error: %v", err)
	}
	defer database.Close()

	// Create WebSocket Hub
	hub := events.NewHub()
	log.Printf("[main] WebSocket Hub ready")

	// Read OpenClaw config for dispatch
	dispatchCfg := readOpenClawConfig()

	// Create agent lifecycle checker
	checker := agentctl.NewChecker(database, hub)

	// Start auto-recovery scheduler (ping all agents every 60s)
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			agents, err := database.ListAgents()
			if err != nil {
				continue
			}
			for _, a := range agents {
				if !a.Enabled {
					continue
				}
				result, err := checker.Ping(&a)
				if err != nil {
					continue
				}
				if !result.Healthy && a.Status != "offline" {
					// Try to wake
					log.Printf("[scheduler] Agent %s unhealthy, attempting wake...", a.Name)
					if wakeErr := checker.Wake(&a); wakeErr != nil {
						log.Printf("[scheduler] Wake %s failed: %v", a.Name, wakeErr)
					}
				}
			}
		}
	}()
	log.Printf("[main] Auto-recovery scheduler started (60s interval)")

	// Start API server with dispatch config (blocks)
	addr := fmt.Sprintf(":%d", *port)
	go func() {
		if err := api.StartWithDispatch(addr, database, hub, dispatchCfg); err != nil {
			log.Printf("[main] Server error: %v", err)
		}
	}()

	log.Printf("[main] MRLI-Agent started on http://localhost:%d", *port)
	if dispatchCfg.OpenClawToken != "" {
		log.Printf("[main] OpenClaw token loaded (chat enabled)")
	}

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[main] Shutting down...")
}

// readOpenClawConfig reads the OpenClaw gateway token from config file.
func readOpenClawConfig() dispatch.Config {
	cfg := dispatch.Config{}

	// Try multiple paths
	home, _ := os.UserHomeDir()
	paths := []string{
		filepath.Join(home, ".openclaw", "openclaw.json"),
		"/mnt/c/Users/26197/.openclaw/openclaw.json",
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}

		var config struct {
			Gateway struct {
				Auth struct {
					Token string `json:"token"`
				} `json:"auth"`
			} `json:"gateway"`
		}

		if err := json.Unmarshal(data, &config); err != nil {
			log.Printf("[main] Failed to parse openclaw config: %v", err)
			continue
		}

		if config.Gateway.Auth.Token != "" {
			cfg.OpenClawToken = config.Gateway.Auth.Token
			log.Printf("[main] Loaded OpenClaw token from %s", p)
			return cfg
		}
	}

	log.Printf("[main] No OpenClaw token found, OpenClaw chat disabled")
	return cfg
}
