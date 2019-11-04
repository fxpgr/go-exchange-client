package main

import (
	"fmt"

	"github.com/fxpgr/go-exchange-client/api/public"
)

func main() {
	cli, err := public.NewClient("kucoin")
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	board, err := cli.Precise("OCN", "BTC")
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	fmt.Println(board)
}
