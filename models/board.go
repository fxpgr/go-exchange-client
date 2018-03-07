package models

type BoardOrder struct {
	Type OrderType
	Price float64
	Amount float64
}

type Board struct {
	Asks []BoardOrder
	Bids []BoardOrder
}