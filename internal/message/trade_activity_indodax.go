package message

import (
	"log"
	"strconv"
	"time"
)

type TradeActivityIndodax struct {
	ExchangeName    string
	PairSymbol      string
	CreatedDatetime time.Time
	Data            map[string]any
}

func (obi *TradeActivityIndodax) FinalizeData() {
	res, ok := obi.Data["result"].(map[string]any)
	if !ok {
		log.Print("[trade_activity_indodax.go] Unable to find 'result' in the data")
	}
	data1, _ := res["data"].(map[string]any)
	data2, _ := data1["data"].([]any)
	trade, ok := data2[0].([]any)
	if !ok {
		log.Printf("[trade_activity_indodax.go] Unable to extract trading activity")
	}
	volume, err := strconv.ParseFloat(trade[6].(string), 64)
	if err != nil {
		log.Printf("[trade_activity_indodax.go] Error converting volume to float64.")
	}
	final_data := &FinalTradeActivityData{
		ExchangeName:    obi.ExchangeName,
		PairSymbol:      obi.PairSymbol,
		OrderType:       trade[3].(string),
		Price:           trade[4].(float64),
		Volume:          volume,
		CreatedDatetime: obi.CreatedDatetime,
	}
	final_data.storeData()
}
