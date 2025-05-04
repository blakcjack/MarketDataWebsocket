package message

import (
	"strconv"
	"time"
)

type OrderBookIndodax struct {
	ExchangeName    string
	PairSymbol      string
	CreatedDatetime time.Time
	Data            map[string]any
}

func (obi *OrderBookIndodax) FinalizeData(volumne_name string) {
	res, _ := obi.Data["result"].(map[string]any)

	data1, _ := res["data"].(map[string]any)

	data2, _ := data1["data"].(map[string]any)

	ask := data2["ask"].([]any)
	// askData := make([]map[string]any, len(ask))
	// for i, v := range ask {
	// 	askData[i] = v.(map[string]any)
	// }
	askMap := ask[0].(map[string]any)
	askPriceStr := askMap["price"].(string)
	askVolStr := askMap[volumne_name].(string)
	askPrice, _ := strconv.ParseFloat(askPriceStr, 64)
	askVol, _ := strconv.ParseFloat(askVolStr, 64)

	if len(ask) > 0 {
		final_data := &FinalOrderBookData{
			ExchangeName:    obi.ExchangeName,
			PairSymbol:      obi.PairSymbol,
			OrderPosition:   "ask",
			Price:           askPrice,
			Volume:          askVol,
			CreatedDatetime: obi.CreatedDatetime,
		}
		_ = final_data.storeData()
	}

	bid := data2["bid"].([]any)
	bidMap := bid[0].(map[string]any)
	priceStr := bidMap["price"].(string)
	volStr := bidMap[volumne_name].(string)
	bidPrice, _ := strconv.ParseFloat(priceStr, 64)
	bidVol, _ := strconv.ParseFloat(volStr, 64)

	if len(bid) > 0 {
		final_data := &FinalOrderBookData{
			ExchangeName:    obi.ExchangeName,
			PairSymbol:      obi.PairSymbol,
			OrderPosition:   "bid",
			Price:           bidPrice,
			Volume:          bidVol,
			CreatedDatetime: obi.CreatedDatetime,
		}
		_ = final_data.storeData()
	}

}
