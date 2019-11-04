package main

import (
	"fmt"

	"github.com/fxpgr/go-exchange-client/api/public"
)

func main() {
	cli, err := public.NewClient("hitbtc")
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	board, err := cli.Precise("XRP", "BTC")
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	fmt.Println(board)
}
