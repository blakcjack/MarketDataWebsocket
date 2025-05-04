package manager

/*
Manager will act as a bridge to manage various process.

* mapping each client and its process

*/

import (
	"context"
	"crypto_websocket/internal/config"
	"crypto_websocket/internal/database"
	"crypto_websocket/internal/websocket"
	"fmt"
	"log"
	"sync"
)

type Manager struct {
	config  *config.Config
	clients map[string]*websocket.WebsocketClient
	Wg      *sync.WaitGroup
	stopCh  chan struct{}
}

// function to create new manager
func InitManager(cfg *config.Config) *Manager {
	return &Manager{
		config:  cfg,
		clients: make(map[string]*websocket.WebsocketClient, len(cfg.Servers)),
		stopCh:  make(chan struct{}),
		Wg:      &sync.WaitGroup{},
	}
}

// let's give our manager ability to start
func (m *Manager) Start(ctx context.Context) error {
	_, err := database.InitDB(m.config.Databases)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	// let's connect to each client server
	for _, serverConfig := range m.config.Servers {
		log.Println("[manager.go] Processing server:", serverConfig.Name)
		client := websocket.InitNewClient(serverConfig)
		if err := client.Connect(ctx, m.Wg); err != nil {
			log.Printf("[manager.go] Error connecting to %s: %v", serverConfig.Name, err)
			continue
		}

		m.clients[serverConfig.Name] = client
		log.Printf("[manager.go] [%s] client %s has been started on URL: %s", client.ServerConfig.Name, client.ServerConfig.Name, client.ServerConfig.Url)

	}

	// // Now, lets listen message from each server
	// for _, client := range m.clients {
	// 	// start the message listener
	// 	m.Wg.Add(1)
	// 	go func() {
	// 		defer m.Wg.Done()
	// 		client.ListenMesages(ctx, m.Wg)
	// 	}()
	// }
	return nil
}

// the manager must be able to stop all the processes on shut down
func (m *Manager) Stop() {
	log.Print("[manager.go] Disconnecting all clients...")

	for name, client := range m.clients {
		log.Printf("[manager.go] Disconnecting from %s", name)
		if err := client.Disconnect(); err != nil {
			log.Printf("[manager.go] Error disconnecting from %s: %v", name, err)
		}
	}

	close(m.stopCh)

	m.Wg.Wait()
}
