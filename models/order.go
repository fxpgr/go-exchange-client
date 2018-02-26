package models

import (
	"time"

	"github.com/jinzhu/gorm"
)

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