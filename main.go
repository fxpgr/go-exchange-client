package main

import (
	"fmt"
	"github.com/fxpgr/go-ccex-api-client/api/private"
	"github.com/fxpgr/go-ccex-api-client/api/public"
	"github.com/fxpgr/go-ccex-api-client/models"
)

func main() {
	cli, err := public.NewClient("bitflyer")
	if err != nil {
		panic(err)
	}
	currencyPairs, err := cli.CurrencyPairs()
	if err != nil {
		panic(err)
	}
	for _, v := range currencyPairs {
		fmt.Println(cli.Rate(v.Trading, v.Settlement))
		fmt.Println(cli.Volume(v.Trading, v.Settlement))
	}
	cli2, err := private.NewClient("bitflyer", "APIKEY", "SECRETKEY")
	if err != nil {
		panic(err)
	}
	fmt.Println(cli2.Balances())
	fmt.Println(cli2.Order("BTC", "USDT", models.Bid, 10000.0, 1))
}
