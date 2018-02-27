# go-ccex-api-client

[![GoDoc](https://img.shields.io/badge/api-Godoc-blue.svg?style=flat-square)](https://godoc.org/github.com/airking05/go-ccex-api-client)

This package presents a client for cryptocoin exchange api.

## Example

```go
package main

import (
	"fmt"
	"github.com/fxpgr/go-ccex-api-client/api"
	"github.com/fxpgr/go-ccex-api-client/models"
)

func main() {
	bitflyerPublicApi,err := api.NewExchangePublicRepository("bitflyer")
	if err != nil {
    		panic(err)
    }
    currencyPairs,err := bitflyerPublicApi.CurrencyPairs()
    if err != nil {
    		panic(err)
    }
    for _,v := range currencyPairs {
    	fmt.Println(bitflyerPublicApi.Rate(v.Trading,v.Settlement))
    	fmt.Println(bitflyerPublicApi.Volume(v.Trading,v.Settlement))
    }
    
    bitflyerPrivateApi,err := api.NewExchangePrivateRepository("bitflyer","APIKEY","SECRETKEY")
    bitflyerPrivateApi.Balances()
    bitflyerPrivateApi.Order("BTC","USDT",models.Bid,10000.0,1)
}
```

## PublicAPI

|          | fetchRate() | Volume() | CurrencyPairs() | Rate() |
|----------|-------------|----------|-----------------|--------|
| Bitflyer | Done        | Done     | Done            | Done   |
| Poloniex | Done        | Done     | Done            | Done   |
| Hitbtc   | Done        | Done     | Done            | Done   |

## PrivateAPI

|          | Order() | CancelOrder() | SellFeeRate() | PurchaseFeeRate() | Balances() | CompleteBalances() | ActiveOrders() | TransferFee() | Transfer() | Address() |
|----------|---------|---------------|---------------|-------------------|------------|--------------------|----------------|---------------|------------|-----------|
| Bitflyer | Done    | Done          | Done          | Done              | Done       | Done               | Done           | Done          | Done       | Done      |
| Poloniex | Done    | Done          | Done          | Done              | Done       | Done               | Done           | Done          | Done       | Done      |