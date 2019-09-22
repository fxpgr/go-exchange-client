package main

import (
	"fmt"

	"github.com/fxpgr/go-exchange-client/api/public"
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
	fmt.Println(board.Asks[0].Amount)
	fmt.Println(len(board.Bids))

}
