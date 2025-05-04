package websocket

import (
	"context"
	"crypto_websocket/internal/config"
	"crypto_websocket/internal/message"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WebsocketClient struct {
	ServerConfig *config.ServerConfig
	conn         *websocket.Conn
	mu           sync.Mutex
	isConnected  bool
	stopCh       chan struct{}
}

type AuthParamsIndodax struct {
	Token string `json:"token"`
}

type ChannelParamsIndodax struct {
	Channel string `json:"channel"`
}

type ChannelParamsTokoCrypto struct {
	Channel []string `json:"channel"`
}

type SubscriptionIndodax struct {
	Method int         `json:"method,omitempty"`
	Params interface{} `json:"params,omitempty"`
	ID     int         `json:"id"`
}

type SubscriptionTokocrypto struct {
	Method string   `json:"method,omitempty"`
	Params []string `json:"params,omitempty"`
	ID     int      `json:"id"`
}

func InitNewClient(config *config.ServerConfig) *WebsocketClient {
	return &WebsocketClient{
		ServerConfig: config,
		stopCh:       make(chan struct{}),
		isConnected:  false,
	}
}

// let's give our client ability to connect
func (c *WebsocketClient) Connect(ctx context.Context, wg *sync.WaitGroup) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isConnected {
		return nil
	}

	header := http.Header{}
	for key, value := range c.ServerConfig.Headers {
		header.Add(key, value)
	}

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.DialContext(ctx, c.ServerConfig.Url, header)

	if err != nil {
		return fmt.Errorf("error connecting to %s: %w", c.ServerConfig.Url, err)
	}
	c.conn = conn
	c.isConnected = true
	log.Printf("[client.go] Connected to %s", c.ServerConfig.Name)

	if err := c.Subscribe(c.ServerConfig, ctx); err != nil {
		log.Printf("[client.go] Error subscribing to channel %s: %v", c.ServerConfig.Name, err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		c.ListenMesages(ctx)
	}()

	return nil
}

// let's add capability to subscribe
func (c *WebsocketClient) Subscribe(cfg *config.ServerConfig, ctx context.Context) error {
	if cfg.Name == "indodax" {
		indodaxSubscription := []SubscriptionIndodax{
			{
				Params: AuthParamsIndodax{Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE5NDY2MTg0MTV9.UR1lBM6Eqh0yWz-PVirw1uPCxe60FdchR8eNVdsskeo"},
				ID:     1,
			},
			{
				Method: 7,
				ID:     3,
			},
		}
		var ID int
		ID = 3
		for _, channel := range cfg.Channels {
			if strings.Contains(channel.Name, "trade") {
				for _, asset := range cfg.Assets {
					ID = ID + 1
					type_1_asset := strings.ReplaceAll(asset.Name, "_", "")
					subscription := SubscriptionIndodax{
						Method: 1,
						Params: ChannelParamsIndodax{Channel: channel.Name + type_1_asset},
						ID:     ID,
					}
					indodaxSubscription = append(indodaxSubscription, subscription)
				}
			}
			if strings.Contains(channel.Name, "order_book") {
				for _, asset := range cfg.Assets {
					ID = ID + 1
					type_1_asset := strings.ReplaceAll(asset.Name, "_", "")
					subscription := SubscriptionIndodax{
						Method: 1,
						Params: ChannelParamsIndodax{Channel: channel.Name + type_1_asset},
						ID:     ID,
					}
					indodaxSubscription = append(indodaxSubscription, subscription)
				}
			}
		}
		for i, sub := range indodaxSubscription {
			subData, err := json.Marshal(sub)
			if err != nil {
				return fmt.Errorf("error marshalling subscription %d: %w", i, err)
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, subData); err != nil {
				return fmt.Errorf("error sending message subscription %d: %w", i, err)
			}

			//wait for subscription response
			_, msg, err := c.conn.ReadMessage()
			if err != nil {
				return fmt.Errorf("can not read subscription response %d: %w", i, err)
			}

			log.Printf("[client.go] [%s] Subscription %d response: %s", cfg.Name, i, string(msg))
		}
	}
	if cfg.Name == "tokoCrypto" {
		tokoCryptoSubscriptions := []SubscriptionTokocrypto{}
		var ID int = 3
		for _, channel := range cfg.Channels {
			if channel.Name == "@trade" {
				for _, asset := range cfg.Assets {
					ID = ID + 1
					type_1_asset := strings.ReplaceAll(asset.Name, "_", "")
					params := ChannelParamsTokoCrypto{Channel: []string{type_1_asset + channel.Name}}
					subscription := SubscriptionTokocrypto{
						Method: "SUBSCRIBE",
						Params: params.Channel,
						ID:     ID,
					}
					tokoCryptoSubscriptions = append(tokoCryptoSubscriptions, subscription)
				}
			}
			if strings.Contains(channel.Name, "depth") {
				for _, asset := range cfg.Assets {
					ID = ID + 1
					type_1_asset := strings.ReplaceAll(asset.Name, "_", "")
					params := ChannelParamsTokoCrypto{Channel: []string{type_1_asset + channel.Name}}
					subscription := SubscriptionTokocrypto{
						Method: "SUBSCRIBE",
						Params: params.Channel,
						ID:     ID,
					}
					tokoCryptoSubscriptions = append(tokoCryptoSubscriptions, subscription)
				}
			}
		}
		subscription := SubscriptionTokocrypto{
			Method: "LIST_SUBSCRIPTIONS",
			ID:     ID + 1,
		}
		tokoCryptoSubscriptions = append(tokoCryptoSubscriptions, subscription)
		for i, sub := range tokoCryptoSubscriptions {
			subData, err := json.Marshal(sub)
			if err != nil {
				return fmt.Errorf("error marshalling subscription %d: %w", i, err)
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, subData); err != nil {
				return fmt.Errorf("error sending message subscription %d: %w", i, err)
			}

			//wait for subscription response
			_, msg, err := c.conn.ReadMessage()
			if err != nil {
				return fmt.Errorf("can not read subscription response %d: %w", i, err)
			}

			log.Printf("[client.go] [%s] Subscription %d response: %s", cfg.Name, i, string(msg))
		}
	}

	return nil
}

// let's give the ability to our client to listen the message from the server
// please note that this function will be executed for each server that we have
func (c *WebsocketClient) ListenMesages(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[client.go] Recovered from panic in message listener: %v", r)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		default:
			_, msg, err := c.conn.ReadMessage()
			if err != nil {
				log.Printf("[%s] Error reading message: %v", c.ServerConfig.Name, err)
				select {
				case <-c.stopCh:
					return
				default:
					c.reconnect()
					return
				}
			}

			rawMsg, err := message.CreateMessage(c.ServerConfig.Name, c.ServerConfig.Url, c.ServerConfig.Assets, msg)
			if err != nil {
				log.Printf("Error creating raw message: %v", err)
			}
			err = rawMsg.HandleMessage(ctx)
			if err != nil {
				log.Printf("[client.go] Error Translating message: %v", err)
			}
		}
	}
}

// let's enable process message
// func (c *WebsocketClient) handleMessages(server string, data []byte) {
// 	var jsonMsg map[string]interface{}
// 	if err := json.Unmarshal(data, &jsonMsg); err != nil {
// 		log.Printf("Error parsing message: %v", err)
// 		return
// 	}

// 	rawMsg := models.CreateMessage(server, c.ServerConfig.Url, jsonMsg)

// 	// log.Printf("The raw message is: %s", rawMsg)

// 	if server == "indodax" {
// 		result, ok := rawMsg.Data["result"].(map[string]interface{})
// 		if !ok {
// 			return
// 		}
// 		channel, ok := result["channel"].(string)
// 		if !ok {
// 			// log.Printf("[%s] No channel found in message: %s", server, string(data))
// 			return
// 		}
// 		// var channelName string = ""
// 		// var coinName string = ""
// 		if strings.HasPrefix(channel, "market:order-book-") {
// 			parts := strings.Split(channel, "-")
// 			rawMsg.ChannelName = "order_book"
// 			var lastPart string = ""
// 			if len(parts) > 0 {
// 				lastPart = parts[len(parts)-1]
// 			}

// 			for _, coin := range c.ServerConfig.Coins {
// 				if strings.ReplaceAll(coin.Coin_name, "_", "") == lastPart {
// 					rawMsg.CoinName = coin.Coin_name
// 				}
// 			}
// 			var orderBook models.OrderBookIndodax
// 			if err := json.Unmarshal(data, &orderBook); err != nil {
// 				log.Printf("[%s] Errror unmaarshalling order book message: %v", server, err)
// 				return
// 			}

// 			// logic to process the message
// 			err := orderBook.ProcessMessage(server)
// 			if err != nil {
// 				log.Printf("[%s] error: %v", server, err)
// 				return
// 			}
// 		}
// 		if strings.HasPrefix(channel, "market:trade-activity-") {
// 			parts := strings.Split(channel, "-")
// 			// channelName = "trade"
// 			var lastPart string = ""
// 			if len(parts) > 0 {
// 				lastPart = parts[len(parts)-1]
// 			}

// 			for _, coin := range c.ServerConfig.Coins {
// 				if strings.ReplaceAll(coin.Coin_name, "_", "") == lastPart {
// 					// coinName = coin.Coin_name
// 				}
// 			}

// 			// logic to store trade data to database
// 		}
// 	} else if server == "tokocrypto" {
// 		// log.Printf("[%s] Processing message: %s", server, data)
// 		ch, ok := jsonMsg["e"].(string)
// 		if !ok {
// 			log.Print(jsonMsg)
// 			return
// 		}
// 		symbol, _ := jsonMsg["s"].(string)
// 		// var channelName string = ""
// 		// var coinName string = ""
// 		if ch == "depthUpdate" {
// 			// channelName = "order_book"
// 			for _, coin := range c.ServerConfig.Coins {
// 				if strings.ReplaceAll(coin.Coin_name, "_", "") == strings.ToLower(symbol) {
// 					// coinName = coin.Coin_name
// 				}
// 			}
// 			// logic to store trade data to database
// 		} else if ch == "trade" {
// 			// channelName = "trade"
// 			for _, coin := range c.ServerConfig.Coins {
// 				if strings.ReplaceAll(coin.Coin_name, "_", "") == strings.ToLower(symbol) {
// 					// coinName = coin.Coin_name
// 				}
// 			}
// 			// logic to store trade data to database
// 		}
// 	}
// }

// Add the capability to reconnect to the server
func (c *WebsocketClient) reconnect() {
	maxRetries := 5
	retryDelay := 5 * time.Second

	for i := 0; i < maxRetries; i++ {
		log.Printf("[client.go] Attempting to reconnect to %s (attempt %d/%d)...", c.ServerConfig.Url, i+1, maxRetries)

		// Ensure we're disconnected
		_ = c.Disconnect()

		time.Sleep(retryDelay)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := c.Connect(ctx, &sync.WaitGroup{})
		defer cancel()

		if err == nil {
			log.Printf("[client.go] Successfully reconnected to %s", c.ServerConfig.Url)
			return
		}

		log.Printf("[client.go] Reconnection attempt failed: %v", err)

		retryDelay *= 2
	}

	log.Printf("[client.go] Failed to reconnect to %s after %d attempts", c.ServerConfig.Url, maxRetries)
}

// let's give our client ability to disconnect itself from the server
func (c *WebsocketClient) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// if the client is not connected, we can directly exit the function
	if !c.isConnected {
		log.Print("[client.go] No client is connected. direct closed!")
		return nil
	}

	// give signal to the listener to stop
	close(c.stopCh)

	if c.conn != nil {
		err := c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Printf("[client.go] Error sending close message: %v", err)
		}

		err = c.conn.Close()
		if err != nil {
			return fmt.Errorf("error closing connection %w", err)
		}
	}

	c.isConnected = false
	log.Printf("[client.go] Disconnected from %s", c.ServerConfig.Url)

	c.stopCh = make(chan struct{})

	return nil
}
