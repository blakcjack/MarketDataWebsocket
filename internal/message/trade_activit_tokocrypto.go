package message

import (
	"log"
)

type TradeActivityIndodax struct {
	Result struct {
		Channel string `json:"channel"`
		Data    struct {
			Data   [][]interface{} `json:"data"`
			Offset int             `json:"offset"`
		}
	} `json:"result"`
}

func (obi *TradeActivityIndodax) StoreData() error {
	log.Print("I am processing trade message")
	return nil
}
