package main

import (
	"context"
	"crypto_websocket/internal/config"
	"crypto_websocket/internal/manager"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log.Println("[main.go] Load the configuration..")
	cfg, err := config.InitConfig()
	if err != nil {
		log.Fatalf("[main.go] Error loading configuration: %v", err)
	}

	// log.Println("Load the databases...")
	// dbs, err := database.InitDB(cfg.Databases, "public")
	// if err != nil {
	// 	log.Fatalf("Error open the connection configuration: %v", err)
	// }
	// defer dbs.PostgresDB_.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Println("[main.go] Initializing the client manager ...")
	manager := manager.InitManager(cfg)
	if err := manager.Start(ctx); err != nil {
		log.Fatalf("[main.go] Error starting websocket clients: %v", err)
	}

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for termination signal
	<-sigChan
	log.Println("[main.go] Received shutdown signal")

	// Give connections time to gracefully close
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	log.Println("[main.go] Stopping the client manager ...")
	manager.Stop()

	// Wait for shutdown to complete or timeout
	<-shutdownCtx.Done()
	log.Println("[main.go] Shutdown complete!")
}
