package main

import (
	"fmt"

	"github.com/fxpgr/go-exchange-client/api/public"
)

func main() {
	exchanges := []string{"huobi", "okex", "poloniex", "kucoin", "hitbtc", "binance"}
	for _, e := range exchanges {
		cli, err := public.NewClient(e)
		if err != nil {
			fmt.Println(err)
			panic(err)
		}
		fmt.Println(cli.OrderBookTickMap())
		fmt.Println(cli.Precise("ZRX", "BTC"))
	}
	/*cli, _ := unified.NewShrimpyApi()
	fmt.Println(cli.GetBoards("bittrex"))
	fmt.Println(cli.GetCurrencyPairs("binance"))
	fmt.Println(cli.GetCurrencys("binance"))*/
}
