package main

import (
	"fmt"
	"github.com/fxpgr/go-exchange-client/api/public"
	"github.com/fxpgr/go-exchange-client/models"
	"net/http"
	"sync"
)

func main() {
	pul:=public.NewProxyUrlList("")

	cli, err := public.NewClient("binance")
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	cli.SetTransport(&http.Transport{Proxy: public.RandomProxyUrl(&pul),MaxIdleConnsPerHost: 16})
	_,err=cli.CurrencyPairs()
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	pairs,err:=cli.CurrencyPairs()
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	zeroCounter := 0
	wg := &sync.WaitGroup{}
	for _,pair := range pairs {
		wg.Add(1)
		go func(p models.CurrencyPair){
			defer wg.Done()
			board, err := cli.Board(p.Trading,p.Settlement)
			if err != nil {
				fmt.Println(err)
				return
			}
			if board.BestBidPrice() == 0 {
				zeroCounter++
			}

		}(pair)
	}
	wg.Wait()
	fmt.Printf("%d %d", len(pairs), zeroCounter)

}
