package message

import (
	"log"
)

type TradeActivityTokoCrypto struct {
	Pair      string
	EventTime int64
	Price     string
	Quantity  string
	TradeTime int64
}

func (obi *TradeActivityTokoCrypto) StoreData() error {
	log.Print("I am processing trade message")
	return nil
}
