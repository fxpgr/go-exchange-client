package models

import (
	"github.com/pkg/errors"
	"sort"
)

type BoardOrder struct {
	Type   OrderType
	Price  float64
	Amount float64
}

type Board struct {
	Asks []BoardOrder
	Bids []BoardOrder
}

func (b *Board) BestBidAmount() float64 {
	if len(b.Bids) == 0 {
		return 0
	}
	sort.Slice(b.Bids, func(i, j int) bool {
		return b.Bids[i].Price > b.Bids[j].Price
	})
	return b.Bids[0].Amount
}

func (b *Board) BestAskAmount() float64 {
	if len(b.Asks) == 0 {
		return 0
	}
	sort.Slice(b.Asks, func(i, j int) bool {
		return b.Asks[i].Price < b.Asks[j].Price
	})
	return b.Asks[0].Amount
}

func (b *Board) BestBidPrice() float64 {
	if len(b.Bids) == 0 {
		return 0
	}
	sort.Slice(b.Bids, func(i, j int) bool {
		return b.Bids[i].Price > b.Bids[j].Price
	})
	return b.Bids[0].Price
}

func (b *Board) BestAskPrice() float64 {
	if len(b.Asks) == 0 {
		return 0
	}
	sort.Slice(b.Asks, func(i, j int) bool {
		return b.Asks[i].Price < b.Asks[j].Price
	})
	return b.Asks[0].Price
}

func (b *Board) AverageBidRate(amount float64) (float64, error) {
	if len(b.Bids) == 0 {
		return 0, errors.New("there is no bids")
	}
	sort.Slice(b.Bids, func(i, j int) bool {
		return b.Bids[i].Price < b.Bids[j].Price
	})
	var sum float64
	remainingAmount := amount
	for _, v := range b.Bids {
		if v.Amount > remainingAmount {
			sum += remainingAmount * v.Price
			return sum / amount, nil
		} else {
			sum += v.Amount * v.Price
			remainingAmount = remainingAmount - v.Amount
		}
	}
	return 0, errors.New("there is not enough board orders")
}

func (b *Board) AverageAskRate(amount float64) (float64, error) {
	if len(b.Asks) == 0 {
		return 0, errors.New("there is no asks")
	}
	sort.Slice(b.Asks, func(i, j int) bool {
		return b.Asks[i].Price < b.Asks[j].Price
	})
	var sum float64
	remainingAmount := amount
	for _, v := range b.Asks {
		if v.Amount > remainingAmount {
			sum += remainingAmount * v.Price
			return sum / amount, nil
		} else {
			sum += v.Amount * v.Price
			remainingAmount = remainingAmount - v.Amount
		}
	}
	return 0, errors.New("there is not enough board orders")
}
