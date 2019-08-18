package main

import (
	"fmt"
	"sync"

	"github.com/fxpgr/go-exchange-client/api/public"
	"github.com/fxpgr/go-exchange-client/models"
)

func main() {
	cli, err := public.NewClient("kucoin")
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
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
