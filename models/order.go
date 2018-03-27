package models

type OrderType int

const (
	Ask OrderType = iota
	Bid
)

type Order struct {
	ExchangeOrderID string
	Type            OrderType
	Trading         string
	Settlement      string
	Price           float64
	Amount          float64
}

type FilledOrderInfo struct {
	Price  float64
	Amount float64
}
