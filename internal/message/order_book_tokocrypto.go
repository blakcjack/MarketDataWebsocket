package message

type OrderBookTokoCrypto struct {
	Pair      string
	EventTime int64
	Bid       []any
	Ask       []any
}

func (obi *OrderBookTokoCrypto) StoreData() error {
	return nil
}
