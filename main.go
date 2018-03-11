package main

import (
	"fmt"
	"github.com/fxpgr/go-ccex-api-client/api/public"
)

func main() {
	cli3, err := public.NewClient("huobi")
	if err != nil {
		panic(err)
	}
	board, err := cli3.Board("WAX", "ETH")
	fmt.Println(board.Bids)
	fmt.Println(board.AverageBuyRate(1600000))

}
