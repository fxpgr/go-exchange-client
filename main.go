package main

import (
	"fmt"
	"github.com/fxpgr/go-exchange-client/api/public"
	"github.com/fxpgr/go-exchange-client/models"
	"sync"
)

func main() {

	cli, err := public.NewClient("binance")
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	pairs,err:=cli.CurrencyPairs()
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	zeroCounter := 0
	wg := &sync.WaitGroup{}
	for _,pair := range pairs {
		wg.Add(1)
		go func(p models.CurrencyPair){
			defer wg.Done()
			board, err := cli.Board(p.Trading,p.Settlement)
			if err != nil {
				fmt.Println(err)
				return
			}
			if board.BestBidPrice() == 0 {
				zeroCounter++
			}

		}(pair)
	}
	wg.Wait()
	fmt.Printf("%d %d", len(pairs), zeroCounter)

/*
	cfg := config.ReadConfig("config.yml")
	privateClient, err := private.NewClient(private.PROJECT, "kucoin", cfg.Kucoin.APIKey, cfg.Kucoin.SecretKey)
	cb, _ := (privateClient.CompleteBalance("BTC"))
	fmt.Println(cb.Available)
	_, err = cli.CurrencyPairs()
	if err != nil {
		panic(err)
	}
		counter := 0
		for _, c := range cs {
			_, err := cli.Board(c.Trading, c.Settlement)
			if err != nil {
				fmt.Println(counter)
				fmt.Println(err)
				continue
			}
			counter++
		}
	*/
}
