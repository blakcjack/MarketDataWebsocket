package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	_ "github.com/mattn/go-sqlite3"
)

// Configuration structure
type Config struct {
	WebsocketURLs []string
	DatabasePath  string
}

// Websocket client structure
type WebsocketClient struct {
	URL           string
	Conn          *websocket.Conn
	Done          chan bool
	MessageChan   chan []byte
	ReconnectChan chan bool
	Subscriptions []Subscription
	ID            string
}

// Subscription structure for different channels
type Subscription struct {
	Method int         `json:"method,omitempty"`
	Params interface{} `json:"params,omitempty"`
	ID     int         `json:"id"`
}

// Authentication subscription
type AuthParams struct {
	Token string `json:"token"`
}

// Channel subscription
type ChannelParams struct {
	Channel string `json:"channel"`
}

// Order book message structure
type OrderBookMessage struct {
	Result struct {
		Channel string `json:"channel"`
		Data    struct {
			Data struct {
				Pair string                   `json:"pair"`
				Ask  []map[string]interface{} `json:"ask"`
				Bid  []map[string]interface{} `json:"bid"`
			} `json:"data"`
			Offset int `json:"offset"`
		} `json:"data"`
	} `json:"result"`
}

func extractVolumeValue(item map[string]interface{}) (float64, error) {
	// Look for any key ending with "_volume" but not "idr_volume"
	for key, value := range item {
		// Check if key ends with "_volume"
		if strings.HasSuffix(key, "_volume") {
			// Exclude if it ends with "idr_volume"
			if strings.HasSuffix(key, "idr_volume") {
				continue
			}

			// Try to convert the value to float64
			if volString, ok := value.(string); ok {
				return strconv.ParseFloat(volString, 64)
			}
		}
	}

	// If no matching volume field is found
	return 0, fmt.Errorf("no valid volume field found in %v", item)
}

// Trade activity message structure
type TradeActivityMessage struct {
	Result struct {
		Channel string `json:"channel"`
		Data    struct {
			Data   [][]interface{} `json:"data"`
			Offset int             `json:"offset"`
		} `json:"data"`
	} `json:"result"`
}

// Database handler structure
type DatabaseHandler struct {
	DB *sql.DB
}

// Main application structure
type Application struct {
	Config          *Config
	Clients         map[string]*WebsocketClient
	DatabaseHandler *DatabaseHandler
	WaitGroup       sync.WaitGroup
	Ctx             context.Context
	Cancel          context.CancelFunc
}

// Initialize database
func initDB(dbPath string) (*DatabaseHandler, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// drop the table first
	_, err = db.Exec("DROP TABLE IF EXISTS order_book;")
	if err != nil {
		return nil, fmt.Errorf("failed to create order_book table: %w", err)
	}

	// Create order_book table
	_, err = db.Exec(`CREATE TABLE order_book (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		datetime TEXT,
		source TEXT,
		pair_symbol TEXT,
		order_type TEXT,
		price REAL,
		volume REAL,
		price_20_pct REAL
	)`)
	if err != nil {
		return nil, fmt.Errorf("failed to create order_book table: %w", err)
	}

	// _, err = db.Exec("DROP TABLE IF EXISTS trade_activity;")
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to create trade_activity table: %w", err)
	// }

	// Create trade_activity table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS trade_activity (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		datetime TEXT,
		source TEXT,
		pair_symbol TEXT,
		order_type TEXT,
		volume REAL,
		price REAL
	)`)
	if err != nil {
		return nil, fmt.Errorf("failed to create trade_activity table: %w", err)
	}

	return &DatabaseHandler{DB: db}, nil
}

// Initialize websocket client
func (app *Application) initWebsocketClient(url, id string, subscriptions []Subscription) *WebsocketClient {
	client := &WebsocketClient{
		URL:           url,
		Done:          make(chan bool),
		MessageChan:   make(chan []byte, 100),
		ReconnectChan: make(chan bool, 1),
		Subscriptions: subscriptions,
		ID:            id,
	}

	app.Clients[id] = client
	return client
}

// Connect to websocket
func (client *WebsocketClient) connect() error {
	conn, _, err := websocket.DefaultDialer.Dial(client.URL, nil)
	if err != nil {
		return fmt.Errorf("error connecting to websocket: %w", err)
	}
	client.Conn = conn
	log.Printf("[%s] Connected to websocket %s", client.ID, client.URL)
	return nil
}

// Subscribe to channels sequentially
func (client *WebsocketClient) subscribe(ctx context.Context) error {
	for i, sub := range client.Subscriptions {
		subData, err := json.Marshal(sub)
		if err != nil {
			return fmt.Errorf("error marshalling subscription %d: %w", i, err)
		}

		if err := client.Conn.WriteMessage(websocket.TextMessage, subData); err != nil {
			return fmt.Errorf("error sending subscription %d: %w", i, err)
		}

		// Wait for subscription response
		_, msg, err := client.Conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("error reading subscription response %d: %w", i, err)
		}

		log.Printf("[%s] Subscription %d response: %s", client.ID, i, string(msg))
	}

	return nil
}

// Listen for messages
func (client *WebsocketClient) listenMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-client.Done:
			return
		default:
			_, message, err := client.Conn.ReadMessage()
			if err != nil {
				log.Printf("[%s] Error reading message: %v", client.ID, err)
				client.ReconnectChan <- true
				return
			}
			client.MessageChan <- message
		}
	}
}

// Process messages
func (app *Application) processMessages(ctx context.Context, client *WebsocketClient) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-client.Done:
			return
		case message := <-client.MessageChan:
			go app.handleMessage(client.ID, message)
		}
	}
}

// Handle reconnection
func (client *WebsocketClient) handleReconnection(ctx context.Context, app *Application) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-client.Done:
			return
		case <-client.ReconnectChan:
			log.Printf("[%s] Attempting to reconnect...", client.ID)

			if client.Conn != nil {
				client.Conn.Close()
			}

			// Exponential backoff for reconnection
			backoff := 1
			maxBackoff := 60

			for {
				if err := client.connect(); err != nil {
					log.Printf("[%s] Reconnection failed: %v. Retrying in %d seconds...", client.ID, err, backoff)
					select {
					case <-ctx.Done():
						return
					case <-time.After(time.Duration(backoff) * time.Second):
						backoff = min(backoff*2, maxBackoff)
						continue
					}
				}

				if err := client.subscribe(ctx); err != nil {
					log.Printf("[%s] Resubscription failed: %v. Retrying connection...", client.ID, err)
					if client.Conn != nil {
						client.Conn.Close()
					}
					continue
				}

				// Restart message listener
				go client.listenMessages(ctx)
				break
			}
		}
	}
}

// Helper function for min value
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Handle received message
func (app *Application) handleMessage(clientID string, message []byte) {
	var genericMsg map[string]interface{}
	if err := json.Unmarshal(message, &genericMsg); err != nil {
		log.Printf("[%s] Error unmarshalling message: %v", clientID, err)
		return
	}

	// Check if this is a result with a channel
	result, ok := genericMsg["result"].(map[string]interface{})
	if !ok {
		// This could be a ping-pong response or other message
		log.Printf("[%s] Received message: %s", clientID, string(message))
		return
	}

	channel, ok := result["channel"].(string)
	if !ok {
		log.Printf("[%s] No channel in message: %s", clientID, string(message))
		return
	}

	// Process message based on channel
	if strings.HasPrefix(channel, "market:order-book-") {
		app.processOrderBookMessage(clientID, message)
	} else if strings.HasPrefix(channel, "market:trade-activity-") {
		app.processTradeActivityMessage(clientID, message)
	} else {
		log.Printf("[%s] Unknown channel %s: %s", clientID, channel, string(message))
	}
}

// Process order book message
func (app *Application) processOrderBookMessage(clientID string, message []byte) {
	var orderBook OrderBookMessage
	if err := json.Unmarshal(message, &orderBook); err != nil {
		log.Printf("[%s] Error unmarshalling order book message: %v", clientID, err)
		return
	}

	channel := orderBook.Result.Channel
	symbol := strings.TrimPrefix(channel, "market:order-book-")

	// Process bid data
	if len(orderBook.Result.Data.Data.Bid) > 0 {
		// Calculate total volume for bids
		var totalVolume float64
		volumes := make([]float64, len(orderBook.Result.Data.Data.Bid))
		prices := make([]float64, len(orderBook.Result.Data.Data.Bid))

		for i, bid := range orderBook.Result.Data.Data.Bid {
			vol, err := extractVolumeValue(bid)
			if err != nil {
				log.Printf("[%s] Error extracting volume: %v", clientID, err)
				continue
			}
			priceStr, ok := bid["price"].(string)
			if !ok {
				log.Printf("[%s] Invalid price format", clientID)
				continue
			}

			price, err := strconv.ParseFloat(priceStr, 64)
			if err != nil {
				log.Printf("[%s] Error parsing price: %v", clientID, err)
				continue
			}

			totalVolume += vol
			volumes[i] = vol
			prices[i] = price
		}

		// Find highest bid price
		highestPrice := prices[0]
		recent_vol := volumes[0]
		for i, price := range prices {
			if price > highestPrice {
				highestPrice = price
				recent_vol = volumes[i]
			}
		}

		// Calculate weighted price for top 20% volume
		targetVolume := totalVolume * 0.2
		var cumulativeVolume, weightedPrice float64

		// Sort by price descending (simple bubble sort)
		for i := 0; i < len(prices); i++ {
			for j := 0; j < len(prices)-i-1; j++ {
				if prices[j] < prices[j+1] {
					prices[j], prices[j+1] = prices[j+1], prices[j]
					volumes[j], volumes[j+1] = volumes[j+1], volumes[j]
				}
			}
		}

		for i := 0; i < len(prices) && cumulativeVolume < targetVolume; i++ {
			volumeToAdd := volumes[i]
			if cumulativeVolume+volumeToAdd > targetVolume {
				volumeToAdd = targetVolume - cumulativeVolume
			}
			weightedPrice += prices[i] * volumeToAdd
			cumulativeVolume += volumeToAdd
		}

		if cumulativeVolume > 0 {
			weightedPrice /= cumulativeVolume
		}

		// Save to database
		_, err := app.DatabaseHandler.DB.Exec(
			`INSERT INTO order_book (datetime, source, pair_symbol, order_type, price, volume, price_20_pct) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			time.Now().Format(time.RFC3339),
			clientID,
			symbol,
			"bid",
			highestPrice,
			recent_vol,
			weightedPrice,
		)
		if err != nil {
			log.Printf("[%s] Error saving bid data: %v", clientID, err)
		}
	}

	// Process ask data
	if len(orderBook.Result.Data.Data.Ask) > 0 {
		// Calculate total volume for asks
		var totalVolume float64
		volumes := make([]float64, len(orderBook.Result.Data.Data.Ask))
		prices := make([]float64, len(orderBook.Result.Data.Data.Ask))

		for i, ask := range orderBook.Result.Data.Data.Ask {
			vol, err := extractVolumeValue(ask)
			if err != nil {
				log.Printf("[%s] Error extracting volume: %v", clientID, err)
				continue
			}
			priceStr, ok := ask["price"].(string)
			if !ok {
				log.Printf("[%s] Invalid price format", clientID)
				continue
			}

			price, err := strconv.ParseFloat(priceStr, 64)
			if err != nil {
				log.Printf("[%s] Error parsing price: %v", clientID, err)
				continue
			}

			totalVolume += vol
			volumes[i] = vol
			prices[i] = price
		}

		// Find lowest ask price
		lowestPrice := prices[0]
		recent_vol := volumes[0]
		for i, price := range prices {
			if price < lowestPrice {
				lowestPrice = price
				recent_vol = volumes[i]
			}
		}

		// Calculate weighted price for top 20% volume
		targetVolume := totalVolume * 0.2
		var cumulativeVolume, weightedPrice float64

		// Sort by price ascending (simple bubble sort)
		for i := 0; i < len(prices); i++ {
			for j := 0; j < len(prices)-i-1; j++ {
				if prices[j] > prices[j+1] {
					prices[j], prices[j+1] = prices[j+1], prices[j]
					volumes[j], volumes[j+1] = volumes[j+1], volumes[j]
				}
			}
		}

		for i := 0; i < len(prices) && cumulativeVolume < targetVolume; i++ {
			volumeToAdd := volumes[i]
			if cumulativeVolume+volumeToAdd > targetVolume {
				volumeToAdd = targetVolume - cumulativeVolume
			}
			weightedPrice += prices[i] * volumeToAdd
			cumulativeVolume += volumeToAdd
		}

		if cumulativeVolume > 0 {
			weightedPrice /= cumulativeVolume
		}

		// Save to database
		_, err := app.DatabaseHandler.DB.Exec(
			`INSERT INTO order_book (datetime, source, pair_symbol, order_type, price, volume, price_20_pct) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			time.Now().Format(time.RFC3339),
			clientID,
			symbol,
			"ask",
			lowestPrice,
			recent_vol,
			weightedPrice,
		)
		if err != nil {
			log.Printf("[%s] Error saving ask data: %v", clientID, err)
		}
	}
}

// Process trade activity message
func (app *Application) processTradeActivityMessage(clientID string, message []byte) {
	var tradeActivity TradeActivityMessage
	if err := json.Unmarshal(message, &tradeActivity); err != nil {
		log.Printf("[%s] Error unmarshalling trade activity message: %v", clientID, err)
		return
	}

	channel := tradeActivity.Result.Channel
	symbol := strings.TrimPrefix(channel, "market:trade-activity-")

	for _, data := range tradeActivity.Result.Data.Data {
		if len(data) < 7 {
			log.Printf("[%s] Trade activity data has unexpected format: %v", clientID, data)
			continue
		}

		// Parse trade activity data
		timestamp, ok := data[1].(float64)
		if !ok {
			log.Printf("[%s] Invalid timestamp format: %v", clientID, data[1])
			continue
		}

		tradeType, ok := data[3].(string)
		if !ok {
			log.Printf("[%s] Invalid trade type format: %v", clientID, data[3])
			continue
		}

		price, ok := data[4].(float64)
		if !ok {
			log.Printf("[%s] Invalid price format: %v", clientID, data[4])
			continue
		}

		volume, ok := data[6].(string)
		if !ok {
			log.Printf("[%s] Invalid volume format: %v", clientID, data[6])
			continue
		}

		// Convert timestamp to datetime
		datetime := time.Unix(int64(timestamp), 0).Format(time.RFC3339)

		// Save to database
		_, err := app.DatabaseHandler.DB.Exec(
			`INSERT INTO trade_activity (datetime, source, pair_symbol, order_type, volume, price) VALUES (?, ?, ?, ?, ?, ?)`,
			datetime,
			clientID,
			symbol,
			tradeType,
			volume,
			price,
		)
		if err != nil {
			log.Printf("[%s] Error saving trade activity data: %v", clientID, err)
		}
	}
}

// Run the application
func (app *Application) Run() error {
	log.Println("Starting application...")

	// Create database handler
	dbHandler, err := initDB(app.Config.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	app.DatabaseHandler = dbHandler
	// defer app.DatabaseHandler.DB.Close()

	// Set up first websocket client
	firstClientSubscriptions := []Subscription{
		{
			Params: AuthParams{Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE5NDY2MTg0MTV9.UR1lBM6Eqh0yWz-PVirw1uPCxe60FdchR8eNVdsskeo"},
			ID:     1,
		},
		{
			Method: 7,
			ID:     3,
		},
		{
			Method: 1,
			Params: ChannelParams{Channel: "market:trade-activity-ethidr"},
			ID:     4,
		},
		{
			Method: 1,
			Params: ChannelParams{Channel: "market:trade-activity-ethusdt"},
			ID:     5,
		},
		{
			Method: 1,
			Params: ChannelParams{Channel: "market:trade-activity-usdtidr"},
			ID:     6,
		},
		{
			Method: 1,
			Params: ChannelParams{Channel: "market:order-book-ethidr"},
			ID:     7,
		},
		{
			Method: 1,
			Params: ChannelParams{Channel: "market:order-book-ethusdt"},
			ID:     8,
		},
		{
			Method: 1,
			Params: ChannelParams{Channel: "market:order-book-usdtidr"},
			ID:     9,
		},
	}

	firstClient := app.initWebsocketClient(app.Config.WebsocketURLs[0], "indodax", firstClientSubscriptions)

	// Connect and subscribe
	if err := firstClient.connect(); err != nil {
		return fmt.Errorf("failed to connect to first websocket: %w", err)
	}

	if err := firstClient.subscribe(app.Ctx); err != nil {
		return fmt.Errorf("failed to subscribe to first websocket channels: %w", err)
	}

	// Start goroutines for the first client
	app.WaitGroup.Add(3)
	go func() {
		defer app.WaitGroup.Done()
		firstClient.listenMessages(app.Ctx)
	}()

	go func() {
		defer app.WaitGroup.Done()
		app.processMessages(app.Ctx, firstClient)
	}()

	go func() {
		defer app.WaitGroup.Done()
		firstClient.handleReconnection(app.Ctx, app)
	}()

	// Set up second websocket client as a placeholder
	// This is just a placeholder for the second websocket URL
	// Uncomment and modify this section when ready to use the second websocket
	/*
		secondClientSubscriptions := []Subscription{
			// Add subscriptions for the second websocket here
		}

		secondClient := app.initWebsocketClient(app.Config.WebsocketURLs[1], "websocket2", secondClientSubscriptions)

		// Connect and subscribe for the second client
		if err := secondClient.connect(); err != nil {
			return fmt.Errorf("failed to connect to second websocket: %w", err)
		}

		if err := secondClient.subscribe(app.Ctx); err != nil {
			return fmt.Errorf("failed to subscribe to second websocket channels: %w", err)
		}

		// Start goroutines for the second client
		app.WaitGroup.Add(3)
		go func() {
			defer app.WaitGroup.Done()
			secondClient.listenMessages(app.Ctx)
		}()

		go func() {
			defer app.WaitGroup.Done()
			app.processMessages(app.Ctx, secondClient)
		}()

		go func() {
			defer app.WaitGroup.Done()
			secondClient.handleReconnection(app.Ctx, app)
		}()
	*/

	log.Println("Application started successfully")
	return nil
}

// Clean shutdown
func (app *Application) Shutdown() {
	log.Println("Shutting down application...")

	// Cancel context to signal all goroutines to stop
	app.Cancel()

	// Close all websocket connections
	for id, client := range app.Clients {
		log.Printf("Closing connection for %s", id)
		if client.Conn != nil {
			client.Conn.Close()
		}
		close(client.Done)
	}

	// Wait for all goroutines to finish
	app.WaitGroup.Wait()

	// Close the database connection at shutdown
	if app.DatabaseHandler != nil && app.DatabaseHandler.DB != nil {
		log.Println("Closing database connection")
		app.DatabaseHandler.DB.Close()
	}

	log.Println("Application shutdown complete")
}

func mains() {
	// Setup configuration
	config := &Config{
		WebsocketURLs: []string{
			"wss://ws3.indodax.com/ws/",         // Replace with actual URL
			"wss://second-websocket-url.com/ws", // Replace with actual URL
		},
		DatabasePath: "D:/trading.db",
	}

	// Create application context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize application
	app := &Application{
		Config:  config,
		Clients: make(map[string]*WebsocketClient),
		Ctx:     ctx,
		Cancel:  cancel,
	}

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the application
	if err := app.Run(); err != nil {
		log.Fatalf("Application failed to start: %v", err)
	}

	// Wait for termination signal
	<-sigChan
	log.Println("Received shutdown signal")

	// Perform clean shutdown
	app.Shutdown()
}
