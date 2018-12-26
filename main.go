package main

import (
	"fmt"
	"github.com/fxpgr/go-exchange-client/api/public"
	"github.com/fxpgr/go-exchange-client/models"
	"net"
	"net/http"
	"sync"
	"time"
)

func main() {
	pul := public.NewProxyUrlList("")

	cli, err := public.NewClient("binance")
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	cli.SetTransport(&http.Transport{
		Proxy:               public.RandomProxyUrl(pul),
		MaxIdleConns:        20,
		MaxIdleConnsPerHost: 20,
		DialContext: (&net.Dialer{
			Timeout:   100000 * time.Millisecond, // 接続タイムアウト時間
			KeepAlive: 100 * time.Millisecond,    // 1TCP接続あたりの持続時間=keeyAlive
		}).DialContext,
	})
	_, err = cli.CurrencyPairs()
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	pairs, err := cli.CurrencyPairs()
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	zeroCounter := 0
	errCounter := 0
	wg := &sync.WaitGroup{}
	for _, pair := range pairs {
		wg.Add(1)
		go func(p models.CurrencyPair) {
			defer func() {
				wg.Done()
			}()
			board, err := cli.Board(p.Trading, p.Settlement)
			if err != nil {
				errCounter++
				return
			}
			if board.BestBidPrice() == 0 {
				zeroCounter++
			}
		}(pair)
	}
	wg.Wait()
	fmt.Printf("%d %d\n", len(pairs), zeroCounter)
	fmt.Printf("%d\n", errCounter)

}
