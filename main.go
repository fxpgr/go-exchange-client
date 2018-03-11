package main

import (
	"fmt"
	"github.com/fxpgr/go-ccex-api-client/api/private"
	"github.com/fxpgr/go-ccex-api-client/api/public"
	"github.com/fxpgr/go-ccex-api-client/models"
)

func main() {
	bitflyerPublicApi, err := public.NewClient("bitflyer")
	if err != nil {
		panic(err)
	}
	currencyPairs, err := bitflyerPublicApi.CurrencyPairs()
	if err != nil {
		panic(err)
	}
	for _, v := range currencyPairs {
		fmt.Println(bitflyerPublicApi.Rate(v.Trading, v.Settlement))
		fmt.Println(bitflyerPublicApi.Volume(v.Trading, v.Settlement))
	}

	bitflyerPrivateApi, err := private.NewClient("bitflyer", "APIKEY", "SECRETKEY")
	bitflyerPrivateApi.Balances()
	bitflyerPrivateApi.Order("BTC", "USDT", models.Bid, 10000.0, 1)
}
