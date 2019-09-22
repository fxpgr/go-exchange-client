package main

import (
	"fmt"
	"sync"

	"github.com/fxpgr/go-exchange-client/api/public"
	"github.com/fxpgr/go-exchange-client/models"
)

func main() {
	cli, err := public.NewClient("binance")
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	board, err := cli.Board("ETH", "BTC")
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	fmt.Println(len(board.Asks))
	fmt.Println(len(board.Bids))
	pairs, err := cli.CurrencyPairs()
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	zeroCounter := 0
	errCounter := 0
	wg := &sync.WaitGroup{}
	for _, pair := range pairs {
		wg.Add(1)
		go func(p models.CurrencyPair) {
			defer func() {
				wg.Done()
			}()
			board, err := cli.Board(p.Trading, p.Settlement)
			if err != nil {
				errCounter++
				return
			}
			if board.BestBidPrice() == 0 {
				zeroCounter++
			}
		}(pair)
	}
	wg.Wait()

	cb, _ := cli.FrozenCurrency()
	fmt.Println(cb)
	fmt.Printf("%d %d\n", len(pairs), zeroCounter)
	fmt.Printf("%d\n", errCounter)

}
