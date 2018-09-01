package main

import (
	"fmt"
	"github.com/fxpgr/go-exchange-client/api/private"
	"github.com/fxpgr/go-exchange-client/api/public"
	"github.com/fxpgr/go-exchange-client/models"
)

func main() {
	pcli, err := private.NewClient(private.TEST, "hitbtc", "", "")
	fmt.Println(pcli.CompleteBalances())
	fmt.Println(pcli.IsOrderFilled("", ""))
	fmt.Println(pcli.Order("", "", models.Bid, 0, 0))

	cli, err := public.NewClient("kucoin")
	if err != nil {
		panic(err)
	}
	if _, err := cli.RateMap(); err != nil {
		panic(err)
	}
	fmt.Println(cli.Rate("ETH", "BTC"))
	board, _ := cli.Board("ETH", "BTC")
	fmt.Println(board.BestBidPrice())
	fmt.Println(board.BestAskPrice())

}
