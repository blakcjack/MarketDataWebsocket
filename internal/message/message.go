package message

import (
	"context"
	"crypto_websocket/internal/config"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

type RawMessage struct {
	serverName string
	serverUrl  string
	data       map[string]any
	coins      []config.Asset
	receivedAt time.Time
}

func CreateMessage(serverName, serverURL string, coin []config.Asset, data []byte) (*RawMessage, error) {
	var jsonMsg map[string]any
	if err := json.Unmarshal(data, &jsonMsg); err != nil {
		log.Printf("[message.go] Error parsing message: %v", err)
		return nil, err
	}

	return &RawMessage{
		serverName: serverName,
		serverUrl:  serverURL,
		data:       jsonMsg,
		coins:      coin,
		receivedAt: time.Now(),
	}, nil
}

func (raw *RawMessage) HandleIndodaxMessage(ctx context.Context) error {
	result, ok := raw.data["result"].(map[string]any)
	if !ok {
		return fmt.Errorf("invalid message format, expecting has result key")
	}
	channel, ok := result["channel"].(string)
	if !ok {
		return fmt.Errorf("invalid message format, result has no channel key")
	}
	var coinName string = ""
	parts := strings.Split(channel, "-")
	var lastPart string = ""
	var volumeName string = ""
	if len(parts) > 0 {
		lastPart = parts[len(parts)-1]
	}
	for _, coin := range raw.coins {
		if strings.ReplaceAll(coin.Name, "_", "") == lastPart {
			coinName = coin.Name
			coinParts := strings.Split(coinName, "_")
			if len(coinParts) > 0 {
				volumeName = coinParts[0] + "_volume"
			}
		}
	}
	if strings.HasPrefix(channel, "market:order-book-") {
		data_ := &OrderBookIndodax{
			ExchangeName:    raw.serverName,
			PairSymbol:      coinName,
			Data:            raw.data,
			CreatedDatetime: raw.receivedAt,
		}
		data_.FinalizeData(volumeName)

		return nil

	} else if strings.HasPrefix(channel, "market:trade-activity-") {

		data_ := &TradeActivityIndodax{
			ExchangeName: raw.serverName,
			PairSymbol:   coinName,
			Data:         raw.data,
		}
		data_.FinalizeData()
		return nil
	}
	return nil
}

func (raw *RawMessage) HandleTokoCryptoMessage(ctx context.Context) error {
	ch, ok := raw.data["e"].(string)
	if !ok {
		return fmt.Errorf("invalid message, it has no key 'e'")
	}
	symbol, _ := raw.data["s"].(string)
	var coinName string = ""
	for _, coin := range raw.coins {
		if strings.ReplaceAll(coin.Name, "_", "") == strings.ToLower(symbol) {
			coinName = coin.Name
		}
	}
	// eventTime, _ := raw.data["E"].(int64)
	// bid, _ := raw.data["b"].([]any)
	// ask, _ := raw.data["a"].([]any)

	if ch == "depthUpdate" {
		data_ := &OrderBookTokoCrypto{
			ExchangeName: raw.serverName,
			PairSymbol:   coinName,
			Data:         raw.data,
		}
		data_.FinalizeData()
	} else if ch == "trade" {
		data_ := &TradeActivityTokoCrypto{
			ExchangeName: raw.serverName,
			PairSymbol:   coinName,
			Data:         raw.data,
		}
		data_.FinalizeData()
	}

	return nil
}
