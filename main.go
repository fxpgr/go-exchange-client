package main

import (
	"fmt"
	"github.com/fxpgr/go-ccex-api-client/api/public"
)

func main() {
	cli, err := public.NewClient("hitbtc")
	if err != nil {
		panic(err)
	}
	fmt.Println(cli.FrozenCurrency())
	fmt.Println(cli.Board("ETH","BTC"))
	cli2, err := public.NewClient("huobi")
	if err != nil {
		panic(err)
	}
	fmt.Println(cli2.Rate("ETH","BTC"))
	/*
	cli3, err := public.NewClient("huobi")
	if err != nil {
		panic(err)
	}
	fmt.Println(cli3.CurrencyPairs())
	rm,err := cli3.RateMap()
	for trading,v := range rm {
		for settlement,rate := range v {
			fmt.Printf("%v %v %v\n",trading,settlement,rate)
		}
	}*/
}
