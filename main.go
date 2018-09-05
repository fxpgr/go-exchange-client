package main

import (
	"fmt"
	"github.com/fxpgr/go-exchange-client/api/public"
)

func main() {
	cli, err := public.NewClient("lbank")
	if err != nil {
		panic(err)
	}
	cs, err := cli.CurrencyPairs()
	if err != nil {
		panic(err)
	}
	counter := 0
	for _, c := range cs {
		_, err := cli.Board(c.Trading, c.Settlement)
		if err != nil {
			fmt.Println(counter)
			fmt.Println(err)
			continue
		}
		counter++
	}

}
