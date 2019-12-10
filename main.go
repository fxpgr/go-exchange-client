package main

import (
	"fmt"

	"github.com/fxpgr/go-exchange-client/api/private"
	"github.com/fxpgr/go-exchange-client/api/public"
)

func main() {
	cli, err := public.NewClient("huobi")
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	fmt.Println(cli.Rate("ZRX", "BTC"))
	fmt.Println(cli.Volume("ZRX", "BTC"))
	fmt.Println(cli.Board("ZRX", "BTC"))
	fmt.Println(cli.OrderBookTickMap())
	fmt.Println(cli.Precise("ZRX", "BTC"))
	pcli, err := private.NewClient(private.PROJECT, "huobi", func() (string, error) { return "", nil }, func() (string, error) { return "", nil })
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	fmt.Println(pcli.TransferFee())
}
