package message

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

type TradeActivity struct {
	ID              uint      `gorm:"primaryKey;column:id"`
	ExchangeName    string    `gorm:"column:exchange_name;type:varchar(50);not null"`
	PairSymbol      string    `gorm:"column:pair_symbol;type:varchar(25);not null"`
	OrderType       string    `gorm:"column:order_type;type:varchar(10);not null"`
	Price           float64   `gorm:"column:price;type:double precision;not null"`
	Volume          float64   `gorm:"column:volume;type:double precision;not null"`
	CreatedDatetime time.Time `gorm:"column:created_datetime;type:timestamp with time zone;default:now()"`
}

func (TradeActivity) TableName() string {
	return "trade_activity"
}

type FinalOrderBookData struct {
	ExchangeName    string
	PairSymbol      string
	OrderPosition   string
	Price           float64
	Volume          float64
	CreatedDatetime time.Time
}

type FinalTradeActivityData struct {
	ExchangeName    string
	PairSymbol      string
	OrderType       string //sell or buy
	Price           float64
	Volume          float64
	CreatedDatetime time.Time
}

func (d *FinalTradeActivityData) ToTradeActivity() TradeActivity {
	return TradeActivity{
		ExchangeName:    d.ExchangeName,
		PairSymbol:      d.PairSymbol,
		OrderType:       d.OrderType,
		Price:           d.Price,
		Volume:          d.Volume,
		CreatedDatetime: d.CreatedDatetime,
	}
}

func SaveTradeActivity(db *gorm.DB, trade *FinalTradeActivityData) (uint, error) {
	// Convert to the ORM model
	tradeActivity := trade.ToTradeActivity()

	// Create the record
	result := db.Create(&tradeActivity)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to insert trade activity: %v", result.Error)
	}

	return tradeActivity.ID, nil
}

func SetupDatabase(db *gorm.DB, schema string) error {
	// Set the schema for this connection
	db = db.Exec(fmt.Sprintf("SET search_path TO %s", schema))

	// Auto migrate will create or update tables based on your struct definitions
	return db.AutoMigrate(&TradeActivity{})
}

func (final *FinalOrderBookData) storeData() error {
	log.Printf("I will save final order book data: %v", final)
	return nil
}

func (final *FinalTradeActivityData) storeData() error {
	log.Print("I will save final trade data: %v", final)
	return nil
}
