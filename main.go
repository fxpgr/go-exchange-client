package main

import (
	"fmt"
	"github.com/fxpgr/go-exchange-client/api/public"

	"github.com/fxpgr/go-arbitrager/infrastructure/config"
	"github.com/fxpgr/go-exchange-client/api/private"
)

func main() {
	cli, err := public.NewClient("kucoin")
	if err != nil {
		panic(err)
	}
	pre, err := cli.Precise("PBL", "BTC")
	if err != nil {
		panic(err)
	}
	fmt.Println(pre)
	cfg := config.ReadConfig("config.yml")
	privateClient, err := private.NewClient(private.PROJECT, "kucoin", cfg.Kucoin.APIKey, cfg.Kucoin.SecretKey)
	tradeFeeRates, err := privateClient.TradeFeeRates()
	fmt.Println(tradeFeeRates)
	fmt.Println(tradeFeeRates["ETH"]["BTC"])
	filled, err := privateClient.IsOrderFilled("ETH", "BTC", "5b92881f9dda152797985c9f")
	if err != nil {
		panic(err)
	}
	fmt.Println(filled)
	_, err = cli.CurrencyPairs()
	if err != nil {
		panic(err)
	}
	/*	counter := 0
		for _, c := range cs {
			_, err := cli.Board(c.Trading, c.Settlement)
			if err != nil {
				fmt.Println(counter)
				fmt.Println(err)
				continue
			}
			counter++
		}
	*/
}
