package main

import (
	"fmt"
	"github.com/fxpgr/go-exchange-client/api/public"

	"github.com/fxpgr/go-arbitrager/infrastructure/config"
	"github.com/fxpgr/go-exchange-client/api/private"
	"github.com/fxpgr/go-exchange-client/models"
	"time"
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

	t := time.Now()
	fmt.Println(privateClient.Order("LOOM", "BTC", models.Bid, 100000000000, 100))
	fmt.Println(time.Now().Sub(t))
	t = time.Now()
	fmt.Println(privateClient.Order("LOOM", "BTC", models.Bid, 100000000000, 100))
	fmt.Println(time.Now().Sub(t))
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
