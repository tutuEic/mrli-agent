package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mrli-agent/internal/agentctl"
	"mrli-agent/internal/api"
	"mrli-agent/internal/db"
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

	// Start API server (blocks)
	go func() {
		if err := api.Start(":8080", database, hub); err != nil {
			log.Printf("[main] Server error: %v", err)
		}
	}()

	log.Printf("[main] MRLI-Agent started on http://localhost:%d", *port)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[main] Shutting down...")
}
