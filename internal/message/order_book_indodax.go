package message

import (
	"log"
)

type OrderBookIndodax struct {
	Result struct {
		Channel string `json:"Channel"`
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

func (obi *OrderBookIndodax) StoreData() error {
	log.Print("I am processing orderbook message")
	return nil
}
