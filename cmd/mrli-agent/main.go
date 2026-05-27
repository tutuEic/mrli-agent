package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

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
