package message

import (
	"fmt"
	"strconv"
)

type OrderBookTokoCrypto struct {
	Pair      string
	EventTime int64
	Bid       []any
	Ask       []any
}

func (obi *OrderBookTokoCrypto) parseBidData() (float64, float64, error) {
	bids := obi.Bid
	var price float64
	var volume float64
	for i, bid := range bids {
		if i == 1 {
			bidData := bid.([]any)
			priceStr := bidData[0].(string)
			volumeStr := bidData[1].(string)

			price, err := strconv.ParseFloat(priceStr, 64)
			if err != nil {
				return 0, 0, fmt.Errorf("failed extracting bid price: %v", err)
			}
			volume, err := strconv.ParseFloat(volumeStr, 64)
			if err != nil {
				return 0, 0, fmt.Errorf("failed extracting bid price: %v", err)
			}
			return price, volume, nil
		}
	}

	return price, volume, nil
}

func (obi *OrderBookTokoCrypto) parseAskData

func (obi *OrderBookTokoCrypto) StoreData() error {
	return nil
}
