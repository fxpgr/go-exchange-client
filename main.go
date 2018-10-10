package main

import (
	"fmt"
	"github.com/fxpgr/go-exchange-client/api/public"
	"net/http"
	"net/url"

	"github.com/fxpgr/go-arbitrager/infrastructure/config"
	"github.com/fxpgr/go-exchange-client/api/private"
)

func main() {
	cli, err := public.NewClient("binance")
	if err != nil {
		panic(err)
	}
	board, err := cli.Board("ETH", "BTC")
	if err != nil {
		panic(err)
	}
	fmt.Println(board)
	fmt.Println(board.BestBidPrice())
	fmt.Println(board.BestAskPrice())
	cli, err = public.NewClient("huobi")
	if err != nil {
		panic(err)
	}
	board, err = cli.Board("ETH", "BTC")
	if err != nil {
		panic(err)
	}
	fmt.Println(board.BestAskPrice())


	proxyURL, err := url.Parse("http://209.126.120.13:9700")
	http.DefaultTransport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}

	h,err:=http.Get("https://google.co.jp")
	if err != nil {
		panic(err)
	}
	fmt.Println(h)
	cfg := config.ReadConfig("config.yml")
	privateClient, err := private.NewClient(private.PROJECT, "kucoin", cfg.Kucoin.APIKey, cfg.Kucoin.SecretKey)
	cb, _ := (privateClient.CompleteBalance("BTC"))
	fmt.Println(cb.Available)
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
