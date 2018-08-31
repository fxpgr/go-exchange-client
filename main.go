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

	cli, err := public.NewClient("lbank")
	if err != nil {
		panic(err)
	}
	fmt.Println(cli.RateMap())
	fmt.Println(cli.Rate("EKO","ETH"))
	board,_ := cli.Board("EKO","ETH")
	fmt.Println(board.BestBidPrice())
	fmt.Println(board.BestAskPrice())

}
