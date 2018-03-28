package main

import (
	"fmt"
	"github.com/fxpgr/go-exchange-client/api/public"
)

func main() {
	cli, err := public.NewClient("cobinhood")
	if err != nil {
		panic(err)
	}
	fmt.Println(cli.RateMap())
	fmt.Println(cli.VolumeMap())
	fmt.Println(cli.Board("ETH", "BTC"))
	fmt.Println(cli.FrozenCurrency())
}
