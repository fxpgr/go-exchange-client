package models

import (
	"sort"
	"github.com/pkg/errors"
)

type BoardOrder struct {
	Type OrderType
	Price float64
	Amount float64
}

type Board struct {
	Asks []BoardOrder
	Bids []BoardOrder
}

func (b *Board) BestBuyPrice(amount float64)(float64,error) {
	sort.Slice(b.Bids, func(i, j int) bool {
		return b.Bids[i].Price < b.Bids[j].Price
	})
	if len(b.Bids) == 0 {
		return 0,errors.New("there is no bids")
	}
	return b.Bids[0].Price,nil
}

func (b *Board) BestSellPrice(amount float64)(float64,error) {
	sort.Slice(b.Asks, func(i, j int) bool {
		return b.Asks[i].Price < b.Asks[j].Price
	})
	if len(b.Asks) == 0 {
		return 0,errors.New("there is no bids")
	}
	return b.Asks[0].Price,nil
}

func (b *Board) AverageBuyRate(amount float64)(float64,error) {
	sort.Slice(b.Bids, func(i, j int) bool {
		return b.Bids[i].Price < b.Bids[j].Price
	})
	if len(b.Bids) == 0 {
		return 0,errors.New("there is no bids")
	}
	var sum float64
	remainingAmount := amount
	for _,v := range b.Bids {
		if v.Amount > remainingAmount {
			sum += remainingAmount * v.Price
			return sum/amount,nil
		} else {
			sum += v.Amount *v.Price
			remainingAmount = remainingAmount - v.Amount
		}
	}
	return 0, errors.New("there is not enough board orders")
}

func (b *Board) AverageSellRate(amount float64)(float64,error) {
	sort.Slice(b.Asks, func(i, j int) bool {
		return b.Asks[i].Price < b.Asks[j].Price
	})
	if len(b.Asks) == 0 {
		return 0,errors.New("there is no asks")
	}
	var sum float64
	remainingAmount := amount
	for _,v := range b.Asks {
		if v.Amount > remainingAmount {
			sum += remainingAmount * v.Price
			return sum/amount,nil
		} else {
			sum += v.Amount *v.Price
			remainingAmount = remainingAmount - v.Amount
		}
	}
	return 0, errors.New("there is not enough board orders")
}