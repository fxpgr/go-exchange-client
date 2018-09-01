package main

import (
	"fmt"
	"github.com/fxpgr/go-exchange-client/api/public"
)

func main() {
	cli, err := public.NewClient("kucoin")
	if err != nil {
		panic(err)
	}
	fmt.Println(cli.CurrencyPairs())
	fmt.Println(cli.Rate("ETH", "BTC"))
	board, _ := cli.Board("ETH", "BTC")
	fmt.Println(board.BestBidPrice())
	fmt.Println(board.BestAskPrice())

}
