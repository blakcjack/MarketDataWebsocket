package message

import (
	"log"
)

type TradeActivityTokoCrypto struct {
	ExchangeName string
	PairSymbol   string
	Data         map[string]any
}

func (obi *TradeActivityTokoCrypto) FinalizeData() {
	log.Print("[trade_activity_tokocrypto.go]")
}
