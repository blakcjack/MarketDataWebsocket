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
	data       []byte
	coins      []config.Asset
	receivedAt time.Time
}

type ProcessedMessage struct {
	ServerName  string
	ServerUrl   string
	Data        any
	CoinName    string
	ChannelName string
	ReceivedAt  time.Time
}

type DataProcessor interface {
	StoreData() error
}

func CreateMessage(serverName, serverURL string, coin []config.Asset, data []byte) (*RawMessage, error) {
	if data == nil {
		return nil, fmt.Errorf("no data found")
	}
	return &RawMessage{
		serverName: serverName,
		serverUrl:  serverURL,
		data:       data,
		coins:      coin,
		receivedAt: time.Now(),
	}, nil
}

// the message must be handled in the raw level
func (raw *RawMessage) HandleMessage(ctx context.Context) error {

	var jsonMsg map[string]any
	if err := json.Unmarshal(raw.data, &jsonMsg); err != nil {
		log.Printf("[message.go] Error parsing message: %v", err)
		return err
	}
	// log.Printf("[message.go] the message is: %v", jsonMsg)

	// var channelName string = ""
	var coinName string = ""
	// var data_ any
	if raw.serverName == "indodax" {
		result, ok := jsonMsg["result"].(map[string]any)
		if !ok {
			return fmt.Errorf("invalid message format, expecting has result key")
		}
		channel, ok := result["channel"].(string)
		if !ok {
			return fmt.Errorf("invalid message format, result has no channel key")
		}

		if strings.HasPrefix(channel, "market:order-book-") {
			// channelName = "order_book"
			var lastPart string = ""
			parts := strings.Split(channel, "-")
			if len(parts) > 0 {
				lastPart = parts[len(parts)-1]
			}
			for _, asset := range raw.coins {
				if strings.ReplaceAll(asset.Name, "_", "") == lastPart {
					coinName = asset.Name
				}
			}
			var data_ *OrderBookIndodax
			if err := json.Unmarshal(raw.data, &data_); err != nil {
				return fmt.Errorf("error unmarshalling order book message for %s from %s order book channel: %v", coinName, raw.serverName, err)
			}
			// dt = &ProcessedMessage{
			// 	ServerName:  raw.serverName,
			// 	ServerUrl:   raw.serverUrl,
			// 	Data:        data_,
			// 	ChannelName: channelName,
			// 	CoinName:    coinName,
			// }s
			return nil
			// log.Printf("Finish printing out the orderbook data: %s", data)
		} else if strings.HasPrefix(channel, "market:trade-activity-") {
			// channelName = "trade"
			parts := strings.Split(channel, "-")
			var lastPart string = ""
			if len(parts) > 0 {
				lastPart = parts[len(parts)-1]
			}
			for _, coin := range raw.coins {
				if strings.ReplaceAll(coin.Name, "_", "") == lastPart {
					coinName = coin.Name
				}
			}
			var data_ *TradeActivityIndodax
			if err := json.Unmarshal(raw.data, &data_); err != nil {
				return fmt.Errorf("error unmarshalling trade activity message for %s from %s order book channel: %v", coinName, raw.serverName, err)
			}
			// return &ProcessedMessage{
			// 	ServerName:  raw.serverName,
			// 	ServerUrl:   raw.serverUrl,
			// 	Data:        data_,
			// 	ChannelName: channelName,
			// 	CoinName:    coinName,
			// },
			return nil
		}
	} else if raw.serverName == "tokoCrypto" {
		ch, ok := jsonMsg["e"].(string)
		if !ok {
			return fmt.Errorf("invalid message, it has no key 'e'")
		}
		symbol, _ := jsonMsg["s"].(string)
		// eventTime, _ := jsonMsg["E"].(int64)
		// bid, _ := jsonMsg["b"].([]any)
		// ask, _ := jsonMsg["a"].([]any)

		if ch == "depthUpdate" {
			// channelName = "order_book"
			for _, coin := range raw.coins {
				if strings.ReplaceAll(coin.Name, "_", "") == strings.ToLower(symbol) {
					coinName = coin.Name
				}
			}
			// var data_ *OrderBookTokoCrypto
			// if err := json.Unmarshal(raw.data, &data_); err != nil {
			// 	return nil, fmt.Errorf("error unmarshalling order book message for %s from %s order book channel: %v", coinName, raw.serverName, err)
			// }

			// data_ := &OrderBookTokoCrypto{
			// 	Pair:      coinName,
			// 	EventTime: eventTime,
			// 	Bid:       bid,
			// 	Ask:       ask,
			// }

			// return &ProcessedMessage{
			// 	ServerName:  raw.serverName,
			// 	ServerUrl:   raw.serverUrl,
			// 	Data:        data_,
			// 	ChannelName: channelName,
			// 	CoinName:    coinName,
			// },

			return nil
		} else if ch == "trade" {
			// channelName = "trade"
			for _, coin := range raw.coins {
				if strings.ReplaceAll(coin.Name, "_", "") == strings.ToLower(symbol) {
					coinName = coin.Name
				}
			}
			// var data_ *TradeActivityTokoCrypto
			// if err := json.Unmarshal(raw.data, &data_); err != nil {
			// 	return nil, fmt.Errorf("error unmarshalling trade activity message for %s from %s trade channel: %v", coinName, raw.serverName, err)
			// }
			// data_ := &TradeActivityTokoCrypto{
			// 	Pair: coinName,
			// 	// Price: ,
			// }
			// return &ProcessedMessage{
			// 	ServerName:  raw.serverName,
			// 	ServerUrl:   raw.serverUrl,
			// 	Data:        data_,
			// 	ChannelName: channelName,
			// 	CoinName:    coinName,
			// },
			return nil
		}
	}

	// return &ProcessedMessage{
	// 	ServerName:  raw.serverName,
	// 	ServerUrl:   raw.serverUrl,
	// 	Data:        data_,
	// 	ChannelName: channelName,
	// 	CoinName:    coinName,
	// },
	return nil
}
