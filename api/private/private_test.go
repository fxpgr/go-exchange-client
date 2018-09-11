package private

import (
	"github.com/fxpgr/go-exchange-client/models"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type FakeRoundTripper struct {
	message  string
	status   int
	header   map[string]string
	requests []*http.Request
}

func (rt *FakeRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	body := strings.NewReader(rt.message)
	rt.requests = append(rt.requests, r)
	res := &http.Response{
		StatusCode: rt.status,
		Body:       ioutil.NopCloser(body),
		Header:     make(http.Header),
	}
	for k, v := range rt.header {
		res.Header.Set(k, v)
	}
	return res, nil
}

func (rt *FakeRoundTripper) Reset() {
	rt.requests = nil
}

func TestNewClient(t *testing.T) {
	_, err := NewClient(TEST, "bitflyer", "APIKEY", "SECRETKEY")
	if err != nil {
		panic(err)
	}
	_, err = NewClient(TEST, "poloniex", "APIKEY", "SECRETKEY")
	if err != nil {
		panic(err)
	}
	_, err = NewClient(TEST, "hitbtc", "APIKEY", "SECRETKEY")
	if err != nil {
		panic(err)
	}
}

func newTestPrivateClient(exchangeName string, rt http.RoundTripper) PrivateClient {
	endpoint := "http://localhost:4243"
	switch strings.ToLower(exchangeName) {
	case "bitflyer":
		n := make(map[string]float64)
		n["JPY"] = 10000
		m := make(map[string]map[string]float64)
		m["BTC"] = n
		return &BitflyerApi{
			BaseURL:           endpoint,
			RateCacheDuration: 30 * time.Second,
			HttpClient:        http.Client{Transport: rt},
			rateMap:           m,
			volumeMap:         nil,
			rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
			m:                 new(sync.Mutex),
		}
	case "poloniex":
		n := make(map[string]float64)
		n["BTC"] = 0.1
		m := make(map[string]map[string]float64)
		m["ETH"] = n
		return &PoloniexApi{
			BaseURL:           endpoint,
			RateCacheDuration: 30 * time.Second,
			HttpClient:        http.Client{Transport: rt},
			rateMap:           m,
			volumeMap:         nil,
			rateLastUpdated:   time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
			m:                 new(sync.Mutex),
		}
	case "hitbtc":
		return &HitbtcApi{
			BaseURL:           endpoint,
			RateCacheDuration: 30 * time.Second,
			HttpClient:        http.Client{Transport: rt},
			settlements:       []string{"BTC"},
			rateMap:           nil,
			volumeMap:         nil,
			rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
			m:                 new(sync.Mutex),
		}
	case "lbank":
		return &LbankApi{
			BaseURL:           endpoint,
			RateCacheDuration: 30 * time.Second,
			HttpClient:        http.Client{Transport: rt},
			settlements:       []string{"BTC"},
			rateMap:           nil,
			volumeMap:         nil,
			rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
			m:                 new(sync.Mutex),
		}
	case "kucoin":
		return &KucoinApi{
			BaseURL:           endpoint,
			RateCacheDuration: 30 * time.Second,
			HttpClient:        &http.Client{Transport: rt},
			settlements:       []string{"BTC"},
			rateMap:           nil,
			volumeMap:         nil,
			rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
			m:                 new(sync.Mutex),
		}
	}
	return nil
}

func TestBitflyerFee(t *testing.T) {
	t.Parallel()
	json := `{
  "commission_rate": 0.001
}`
	rt := &FakeRoundTripper{message: json, status: http.StatusOK}
	client := newTestPrivateClient("bitflyer", rt)
	fee, err := client.TradeFeeRate("BTC", "JPY")
	if err != nil {
		panic(err)
	}
	if fee.MakerFee != 0.001 || fee.TakerFee != 0.001 {
		t.Errorf("PoloniexPrivateApi: Expected %v %v. Got %v %v", 0.001, 0.001, fee.MakerFee, fee.TakerFee)
	}
	_, err = client.TransferFee()
	if err != nil {
		panic(err)
	}
}

func TestBitflyerBalances(t *testing.T) {
	t.Parallel()
	json := `[
  {
    "currency_code": "JPY",
    "amount": 1024078,
    "available": 508000
  },
  {
    "currency_code": "BTC",
    "amount": 10.24,
    "available": 4.12
  },
  {
    "currency_code": "ETH",
    "amount": 20.48,
    "available": 16.38
  }
]`
	client := newTestPrivateClient("bitflyer", &FakeRoundTripper{message: json, status: http.StatusOK})
	balances, err := client.Balances()
	if err != nil {
		panic(err)
	}
	if balances["BTC"] != 4.12 {
		t.Errorf("BitflyerPrivateApi: Expected %v. Got %v", 4.12, balances)
	}
	balanceMap, err := client.CompleteBalances()
	if err != nil {
		panic(err)
	}

	if balanceMap["BTC"].Available != 4.12 || balanceMap["BTC"].OnOrders != 6.12 {
		t.Error("BitflyerPrivateApi: balance map error")
	}
}

func TestBitflyerOrders(t *testing.T) {
	t.Parallel()
	json := `[
  {
    "id": 138398,
    "child_order_id": "JOR20150707-084555-022523",
    "product_code": "BTC_JPY",
    "side": "BUY",
    "child_order_type": "LIMIT",
    "price": 30000,
    "average_price": 30000,
    "size": 0.1,
    "child_order_state": "COMPLETED",
    "expire_date": "2015-07-14T07:25:52",
    "child_order_date": "2015-07-07T08:45:53",
    "child_order_acceptance_id": "JRF20150707-084552-031927",
    "outstanding_size": 0,
    "cancel_size": 0,
    "executed_size": 0.1,
    "total_commission": 0
  }]`
	client := newTestPrivateClient("bitflyer", &FakeRoundTripper{message: json, status: http.StatusOK})
	orders, err := client.ActiveOrders()
	if err != nil {
		panic(err)
	}
	if orders[0].Settlement != "JPY" {
		t.Errorf("BitflyerPrivateApi: Expected %v. Got %v", "JPY", orders[0].Settlement)
	}
	if orders[0].Trading != "BTC" {
		t.Errorf("BitflyerPrivateApi: Expected %v. Got %v", "BTC", orders[0].Trading)
	}
	if orders[0].ExchangeOrderID != "JRF20150707-084552-031927" {
		t.Errorf("BitflyerPrivateApi: Expected %v. Got %v", "JRF20150707-084552-031927", orders[0].ExchangeOrderID)
	}
	if orders[0].Type != models.Ask {
		t.Errorf("BitflyerPrivateApi: Expected %v. Got %v", "BUY", orders[0].Type)
	}
}

func TestBitflyerOrder(t *testing.T) {
	t.Parallel()
	json := `{
    "child_order_acceptance_id": "JRF20150707-050237-639234"
}`
	client := newTestPrivateClient("bitflyer", &FakeRoundTripper{message: json, status: http.StatusOK})
	orderId, err := client.Order("BTC", "JPY", models.Bid, 1000000, 0.01)
	if err != nil {
		panic(err)
	}
	if orderId != "JRF20150707-050237-639234" {
		t.Errorf("BitflyerPrivateApi: Expected %v. Got %v", "JRF20150707-050237-639234", orderId)
	}
	err = client.CancelOrder("BTC", "JPY", models.Bid, orderId)
	if err != nil {
		t.Error(err)
	}
}

func TestBitflyerOthers(t *testing.T) {
	t.Parallel()
	json := ``
	client := newTestPrivateClient("bitflyer", &FakeRoundTripper{message: json, status: http.StatusOK})
	if client.Transfer("", "", 0.1, 0.001) == nil {
		t.Errorf("transfer should not be implemented")
	}
	if _, err := client.Address(""); err == nil {
		t.Errorf("address should not be implemented")
	}
}

func TestPoloniexFee(t *testing.T) {
	t.Parallel()
	json := `{"makerFee": "0.00140000", "takerFee": "0.00240000", "thirtyDayVolume": "612.00248891", "nextTier": "1200.00000000"}`
	rt := &FakeRoundTripper{message: json, status: http.StatusOK}
	client := newTestPrivateClient("poloniex", rt)
	fee, err := client.TradeFeeRate("ETH", "BTC")
	if err != nil {
		panic(err)
	}
	if fee.MakerFee != 0.0014 || fee.TakerFee != 0.0024 {
		t.Errorf("PoloniexPrivateApi: Expected %v %v. Got %v %v", 0.0014, 0.0024, fee.MakerFee, fee.TakerFee)
	}
	rt.message = `{"1CR":{"id":1,"name":"1CRedit","txFee":"0.01000000","minConf":3,"depositAddress":null,"disabled":0,"delisted":1,"frozen":0},"ABY":{"id":2,"name":"ArtByte","txFee":"0.01000000","minConf":8,"depositAddress":null,"disabled":0,"delisted":1,"frozen":0}}`
	_, err = client.TransferFee()
	if err != nil {
		panic(err)
	}
}

func TestPoloniexBalances(t *testing.T) {
	t.Parallel()
	json := `{"BTC":"0.59098578","LTC":"3.31117268"}`
	rt := &FakeRoundTripper{message: json, status: http.StatusOK}
	client := newTestPrivateClient("poloniex", rt)
	balances, err := client.Balances()
	if err != nil {
		panic(err)
	}
	if balances["BTC"] != 0.59098578 {
		t.Errorf("PoloniexPrivateApi: Expected %v. Got %v", 4.12, balances)
	}
	rt.message = `{"LTC":{"available":"5.015","onOrders":"1.0025","btcValue":"0.078"},"NXT":{"available":"5.015","onOrders":"1.0025","btcValue":"0.078"}}`
	balanceMap, err := client.CompleteBalances()
	if err != nil {
		panic(err)
	}

	if balanceMap["LTC"].Available != 5.015 || balanceMap["LTC"].OnOrders != 1.0025 {
		t.Error("PoloniexPrivateApi: balance map error")
	}
}

func TestPoloniexOrders(t *testing.T) {
	t.Parallel()
	json := `{"BTC_AC":[{"orderNumber":"120466","type":"sell","rate":"0.025","amount":"100","total":"2.5"},{"orderNumber":"120467","type":"sell","rate":"0.04","amount":"100","total":"4"}]}`
	client := newTestPrivateClient("poloniex", &FakeRoundTripper{message: json, status: http.StatusOK})
	orders, err := client.ActiveOrders()
	if err != nil {
		panic(err)
	}
	if orders[0].Settlement != "BTC" {
		t.Errorf("PoloniexPrivateApi: Expected %v. Got %v", "BTC", orders[0].Settlement)
	}
	if orders[0].Trading != "AC" {
		t.Errorf("PoloniexPrivateApi: Expected %v. Got %v", "AC", orders[0].Trading)
	}
	if orders[0].ExchangeOrderID != "120466" {
		t.Errorf("PoloniexPrivateApi: Expected %v. Got %v", "120466", orders[0].ExchangeOrderID)
	}
	if orders[0].Type != models.Ask {
		t.Errorf("PoloniexPrivateApi: Expected %v. Got %v", "SELL", orders[0].Type)
	}
}

func TestPoloniexOrder(t *testing.T) {
	t.Parallel()
	json := `{"orderNumber":31226040,"resultingTrades":[{"amount":"338.8732","date":"2014-10-18 23:03:21","rate":"0.00000173","total":"0.00058625","tradeID":"16164","type":"buy"}]}`
	rt := &FakeRoundTripper{message: json, status: http.StatusOK}
	client := newTestPrivateClient("poloniex", rt)
	orderId, err := client.Order("ETH", "BTC", models.Bid, 1000000, 0.01)
	if err != nil {
		panic(err)
	}
	if orderId != "31226040" {
		t.Errorf("PoloniexPrivateApi: Expected %v. Got %v", "31226040", orderId)
	}
	rt.message = `{"success":1}`
	err = client.CancelOrder("ETH", "BTC", models.Bid, orderId)
	if err != nil {
		t.Error(err)
	}
}

func TestPoloniexOthers(t *testing.T) {
	t.Parallel()
	json := `{"response":"Withdrew 2398 NXT."}`
	rt := &FakeRoundTripper{message: json, status: http.StatusOK}
	client := newTestPrivateClient("poloniex", rt)
	if client.Transfer("", "", 0.1, 0.001) != nil {
		t.Errorf("transfer should not be implemented")
	}
	rt.message = `{"BTC":"19YqztHmspv2egyD6jQM3yn81x5t5krVdJ","LTC":"LPgf9kjv9H1Vuh4XSaKhzBe8JHdou1WgUB"}`
	if _, err := client.Address("LTC"); err != nil {
		t.Errorf("address should not be implemented")
	}
}

func TestHitbtcBalances(t *testing.T) {
	t.Parallel()
	json := `[{"currency": "ETH", "available": "10.000000000", "reserved":"0.560000000"},{"currency": "BTC","available":"0.010205869","reserved": "0"}]`
	rt := &FakeRoundTripper{message: json, status: http.StatusOK}
	client := newTestPrivateClient("hitbtc", rt)
	balances, err := client.Balances()
	if err != nil {
		t.Fatal(err)
	}
	if balances["ETH"] != 10.000000000 {
		t.Errorf("HitbtcPrivateApi: Expected %v. Got %v", 10.000000000, balances["ETH"])
	}
	balanceMap, err := client.CompleteBalances()
	if err != nil {
		panic(err)
	}

	if balanceMap["ETH"].Available != 10 || balanceMap["ETH"].OnOrders != 0.56 {
		t.Error("HitbtcPrivateApi: balance map error")
	}
}

func TestHitbtcOrders(t *testing.T) {
	t.Parallel()
	json := `[
  {
    "id": 840450210,
    "clientOrderId": "c1837634ef81472a9cd13c81e7b91401",
    "symbol": "ETHBTC",
    "side": "buy",
    "status": "partiallyFilled",
    "type": "limit",
    "timeInForce": "GTC",
    "quantity": "0.020",
    "price": "0.046001",
    "cumQuantity": "0.005",
    "createdAt": "2017-05-12T17:17:57.437Z",
    "updatedAt": "2017-05-12T17:18:08.610Z"
  }
]`
	client := newTestPrivateClient("hitbtc", &FakeRoundTripper{message: json, status: http.StatusOK})
	orders, err := client.ActiveOrders()
	if err != nil {
		t.Fatal(err)
	}
	if orders[0].Settlement != "BTC" {
		t.Errorf("HitbtcPrivateApi: Expected %v. Got %v", "BTC", orders[0].Settlement)
	}
	if orders[0].Trading != "ETH" {
		t.Errorf("HitbtcPrivateApi: Expected %v. Got %v", "ETH", orders[0].Trading)
	}
	if orders[0].ExchangeOrderID != "c1837634ef81472a9cd13c81e7b91401" {
		t.Errorf("HitbtcPrivateApi: Expected %v. Got %v", "c1837634ef81472a9cd13c81e7b91401", orders[0].ExchangeOrderID)
	}
	if orders[0].Type != models.Ask {
		t.Errorf("HitbtcPrivateApi: Expected %v. Got %v", "BUY", orders[0].Type)
	}
}

func TestHitbtcOrder(t *testing.T) {
	t.Parallel()
	json := `{
        "id": 0,
        "clientOrderId": "d8574207d9e3b16a4a5511753eeef175",
        "symbol": "ETHBTC",
        "side": "sell",
        "status": "new",
        "type": "limit",
        "timeInForce": "GTC",
        "quantity": "0.063",
        "price": "0.046016",
        "cumQuantity": "0.000",
        "createdAt": "2017-05-15T17:01:05.092Z",
        "updatedAt": "2017-05-15T17:01:05.092Z"
    }`
	rt := &FakeRoundTripper{message: json, status: http.StatusOK}
	client := newTestPrivateClient("hitbtc", rt)
	orderId, err := client.Order("ETH", "BTC", models.Bid, 1000000, 0.01)
	if err != nil {
		panic(err)
	}
	if orderId != "d8574207d9e3b16a4a5511753eeef175" {
		t.Errorf("HitbtcPrivateApi: Expected %v. Got %v", "d8574207d9e3b16a4a5511753eeef175", orderId)
	}
	rt.message = ``
	err = client.CancelOrder("ETH", "BTC", models.Bid, orderId)
	if err != nil {
		t.Error(err)
	}
}

func TestHitbtcOthers(t *testing.T) {
	t.Parallel()
	json := `{
  "id": "d2ce578f-647d-4fa0-b1aa-4a27e5ee597b"
}`
	rt := &FakeRoundTripper{message: json, status: http.StatusOK}
	client := newTestPrivateClient("hitbtc", rt)
	if err := client.Transfer("BTC", "test_id", 0.1, 0.001); err != nil {
		t.Fatal(err)
	}
	rt.message = `{
  "address": "NXT-G22U-BYF7-H8D9-3J27W",
  "paymentId": "616598347865"
}`
	if _, err := client.Address("LTC"); err != nil {
		t.Fatal(err)
	}
}

func TestLbankOrder(t *testing.T) {
	t.Parallel()
	json := `{
  "result":"true",
  "order_id":"123456789",
  "success":"*****,*****,*****",
  "error":"*****,*****"
}`
	rt := &FakeRoundTripper{message: json, status: http.StatusOK}
	client := newTestPrivateClient("lbank", rt)
	orderId, err := client.Order("ETH", "BTC", models.Bid, 1000000, 0.01)
	if err != nil {
		panic(err)
	}
	if orderId != "123456789" {
		t.Errorf("LbankPrivateApi: Expected %v. Got %v", "123456789", orderId)
	}
	rt.message = ``
	err = client.CancelOrder("ETH", "BTC", models.Bid, orderId)
	if err != nil {
		t.Error(err)
	}
}

func TestLbankBalances(t *testing.T) {
	t.Parallel()
	json := `{"result":"true","info":{"freeze":{"btc":1,"zec":0,"cny":80000},"asset":{"net":95678.25},"free":{"btc":2,"zec":0,"cny":34}}}`
	rt := &FakeRoundTripper{message: json, status: http.StatusOK}
	client := newTestPrivateClient("lbank", rt)
	balances, err := client.Balances()
	if err != nil {
		t.Fatal(err)
	}
	if balances["BTC"] != 2.000000000 {
		t.Errorf("LbankPrivateApi: Expected %v. Got %v", 2.000000000, balances["BTC"])
	}
	balanceMap, err := client.CompleteBalances()
	if err != nil {
		panic(err)
	}

	if balanceMap["BTC"].Available != 2.0 || balanceMap["BTC"].OnOrders != 1.0 {
		t.Error("LbankPrivateApi: balance map error")
	}
}

func TestKucoinOrder(t *testing.T) {
	t.Parallel()
	jsonPrecision := `{"success":true,"code":"OK","msg":"Operation succeeded.","timestamp":1536683680454,"data":[{"withdrawMinFee":0.5,"coinType":"ERC20","withdrawMinAmount":10.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Kucoin Shares","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"KCS"},{"withdrawMinFee":100.0,"coinType":"ERC20","withdrawMinAmount":500.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://explore.veforge.com/transactions/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Vechain","tradePrecision":8,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"VET"},{"withdrawMinFee":25.0,"coinType":"ERC20","withdrawMinAmount":100.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"AXpire","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"AXPR"},{"withdrawMinFee":1000.0,"coinType":"ERC20","withdrawMinAmount":1.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"EPRX","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":false,"coin":"EPRX"},{"withdrawMinFee":5.0E-4,"coinType":null,"withdrawMinAmount":0.002,"withdrawRemark":"","orgAddress":null,"txUrl":"https://blockchain.info/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":3,"infoUrl":null,"enable":true,"name":"Bitcoin","tradePrecision":8,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"BTC"},{"withdrawMinFee":0.01,"coinType":"ETH","withdrawMinAmount":0.1,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Ethereum","tradePrecision":7,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"ETH"},{"withdrawMinFee":0.0,"coinType":"NEO","withdrawMinAmount":1.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"http://state.otcgo.cn/traninfo.html?id=0x{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":6,"infoUrl":null,"enable":true,"name":"NEO","tradePrecision":6,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"NEO"},{"withdrawMinFee":20.0,"coinType":"ERC20","withdrawMinAmount":100.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"I-House Token","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"IHT"},{"withdrawMinFee":50.0,"coinType":"ERC20","withdrawMinAmount":400.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"TRAXIA","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"TMT"},{"withdrawMinFee":1.0,"coinType":"ERC20","withdrawMinAmount":10.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Fortuna","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"FOTA"},{"withdrawMinFee":0.05,"coinType":null,"withdrawMinAmount":5.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://raiblocks.net/block/index.php?h={txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"NANO","tradePrecision":6,"depositRemark":"","enableWithdraw":true,"enableDeposit":true,"coin":"NANO"},{"withdrawMinFee":2.0,"coinType":"ERC20","withdrawMinAmount":15.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Arcblock","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"ABT"},{"withdrawMinFee":40.0,"coinType":"ERC20","withdrawMinAmount":80.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"SingularityNET","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"AGI"},{"withdrawMinFee":2.0,"coinType":"ERC20","withdrawMinAmount":10.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Aigang","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"AIX"},{"withdrawMinFee":40.0,"coinType":"ERC20","withdrawMinAmount":90.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Enjin Coin","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"ENJ"},{"withdrawMinFee":3.0,"coinType":"ERC20","withdrawMinAmount":20.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Experty","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"EXY"},{"withdrawMinFee":5.0,"coinType":"ERC20","withdrawMinAmount":30.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Havven","tradePrecision":8,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"HAV"},{"withdrawMinFee":0.5,"coinType":"ERC20","withdrawMinAmount":5.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"High Performance Blockch","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"HPB"},{"withdrawMinFee":30.0,"coinType":"ERC20","withdrawMinAmount":200.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"JET8","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"J8T"},{"withdrawMinFee":1.0,"coinType":"NEP","withdrawMinAmount":10.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"http://state.otcgo.cn/traninfo.html?id=0x{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Red Pulse Phoenix","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"PHX"},{"withdrawMinFee":10.0,"coinType":"NEP","withdrawMinAmount":2000.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"http://state.otcgo.cn/traninfo.html?id=0x{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"THEKEY","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"TKY"},{"withdrawMinFee":1.0,"coinType":"NEP","withdrawMinAmount":10.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"http://state.otcgo.cn/traninfo.html?id=0x{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Trinity","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"TNC"},{"withdrawMinFee":0.7,"coinType":null,"withdrawMinAmount":4.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://explorer.wanchain.org/block/trans/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Wanchain","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"WAN"},{"withdrawMinFee":1.0,"coinType":"NEP","withdrawMinAmount":20.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"http://state.otcgo.cn/traninfo.html?id=0x{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Zeepin","tradePrecision":4,"depositRemark":null,"enableWithdraw":false,"enableDeposit":false,"coin":"ZPT"},{"withdrawMinFee":10.0,"coinType":"NEP","withdrawMinAmount":300.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"http://state.otcgo.cn/traninfo.html?id=0x{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Alphacat","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"ACAT"},{"withdrawMinFee":100.0,"coinType":"ERC20","withdrawMinAmount":200.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"DOCK","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"DOCK"},{"withdrawMinFee":130.0,"coinType":"ERC20","withdrawMinAmount":260.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"EthLend","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"LEND"},{"withdrawMinFee":0.1,"coinType":"ERC20","withdrawMinAmount":1.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Chronobank","tradePrecision":6,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"TIME"},{"withdrawMinFee":3.0,"coinType":"ERC20","withdrawMinAmount":20.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Credits","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"CS"},{"withdrawMinFee":1.0,"coinType":"gochain","withdrawMinAmount":200.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://explorer.gochain.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"GoChain","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"GO"},{"withdrawMinFee":0.1,"coinType":"UT","withdrawMinAmount":50.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://explorer.ulord.one/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"UlordToken","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"UT"},{"withdrawMinFee":1.0,"coinType":null,"withdrawMinAmount":100.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://browser.achain.com/#/tradeInfo/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"AChain","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"ACT"},{"withdrawMinFee":3.0,"coinType":"NEP","withdrawMinAmount":25.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"http://state.otcgo.cn/traninfo.html?id=0x{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Aphelion","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"APH"},{"withdrawMinFee":6.0,"coinType":"ERC20","withdrawMinAmount":10.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Aeron","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"ARN"},{"withdrawMinFee":3.0,"coinType":"ERC20","withdrawMinAmount":20.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Bread","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"BRD"},{"withdrawMinFee":1.0,"coinType":"ERC20","withdrawMinAmount":10.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"CanYa","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"CAN"},{"withdrawMinFee":20.0,"coinType":"ERC20","withdrawMinAmount":100.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"BitClave","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"CAT"},{"withdrawMinFee":5.0,"coinType":"ERC20","withdrawMinAmount":50.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"CashBet Coin","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"CBC"},{"withdrawMinFee":25.0,"coinType":"ERC20","withdrawMinAmount":170.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"CoinPoker","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"CHP"},{"withdrawMinFee":10.0,"coinType":"ERC20","withdrawMinAmount":200.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"CPChain","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"CPC"},{"withdrawMinFee":30.0,"coinType":"ERC20","withdrawMinAmount":100.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"CargoX","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"CXO"},{"withdrawMinFee":80.0,"coinType":"ERC20","withdrawMinAmount":500.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Constellation","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"DAG"},{"withdrawMinFee":20.0,"coinType":"ERC20","withdrawMinAmount":100.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Datum","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"DAT"},{"withdrawMinFee":1.0,"coinType":"NEP","withdrawMinAmount":5.0,"withdrawRemark":"","orgAddress":null,"txUrl":"http://state.otcgo.cn/traninfo.html?id=0x{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"DeepBrain Chain","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"DBC"},{"withdrawMinFee":60.0,"coinType":"ERC20","withdrawMinAmount":500.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Distributed Credit Chain","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"DCC"},{"withdrawMinFee":3.0,"coinType":"ERC20","withdrawMinAmount":20.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"EncrypGen","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"DNA"},{"withdrawMinFee":100.0,"coinType":"ERC20","withdrawMinAmount":2000.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"DATA","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"DTA"},{"withdrawMinFee":200.0,"coinType":"ERC20","withdrawMinAmount":1500.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Egretia","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"EGT"},{"withdrawMinFee":0.1,"coinType":"ELA","withdrawMinAmount":0.8,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://blockchain.elastos.org/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Elastos","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"ELA"},{"withdrawMinFee":50.0,"coinType":null,"withdrawMinAmount":350.0,"withdrawRemark":"","orgAddress":"etnjvMPvGM68TZp59KDfWvQoNS7NFaEc9296CK4pSrfA4j5KgTguz5sZNEwC2KMW6iEn8gML7vASrT3gLxD3n9zs3M2PxMYVyH","txUrl":"https://blockexplorer.electroneum.com/tx/{txId}","userAddressName":"Payment ID","withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Electroneum","tradePrecision":2,"depositRemark":"Please specify payment_id for deposit, or the deposit will fail to be credited. | 充值时请务必指定payment_id，否则将无法入账。","enableWithdraw":true,"enableDeposit":true,"coin":"ETN"},{"withdrawMinFee":5.0,"coinType":"ERC20","withdrawMinAmount":12.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"INS Ecosystem","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"INS"},{"withdrawMinFee":5.0,"coinType":"ERC20","withdrawMinAmount":100.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Jibrel Network","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"JNT"},{"withdrawMinFee":3.0,"coinType":"ERC20","withdrawMinAmount":20.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"LockTrip","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"LOC"},{"withdrawMinFee":20.0,"coinType":"ERC20","withdrawMinAmount":700.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Lympo","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"LYM"},{"withdrawMinFee":3.0,"coinType":"ERC20","withdrawMinAmount":20.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"OneLedger","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"OLT"},{"withdrawMinFee":50.0,"coinType":"ERC20","withdrawMinAmount":300.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Shivom","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"OMX"},{"withdrawMinFee":1.0,"coinType":"ONT","withdrawMinAmount":2.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://explorer.ont.io/transaction/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Ontology","tradePrecision":4,"depositRemark":"Notice: KuCoin will automatically swap the ONT token into the MainNet ONT coin before 23:59:59, 2018.09.30 (UTC+8) for users who deposit the ONT token","enableWithdraw":true,"enableDeposit":true,"coin":"ONT"},{"withdrawMinFee":50.0,"coinType":"ERC20","withdrawMinAmount":100.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"QuarkChain","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"QKC"},{"withdrawMinFee":1.0,"coinType":"NEP","withdrawMinAmount":5.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"http://state.otcgo.cn/traninfo.html?id=0x{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Qlink","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"QLC"},{"withdrawMinFee":5.0,"coinType":"ERC20","withdrawMinAmount":40.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"SIRIN LABS Token","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"SRN"},{"withdrawMinFee":500.0,"coinType":"ERC20","withdrawMinAmount":5000.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Telcoin","tradePrecision":2,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"TEL"},{"withdrawMinFee":100.0,"coinType":"ERC20","withdrawMinAmount":900.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"TE-FOOD","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"TFD"},{"withdrawMinFee":1.0,"coinType":"TRX","withdrawMinAmount":10.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://tronscan.org/#/transaction/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Tron","tradePrecision":4,"depositRemark":"KuCoin will continue to support deposits of TRON (TRX) tokens to the original ERC-20 address. The TRX tokens held on KuCoin will automatically be swapped into the MainNet TRX coins.","enableWithdraw":true,"enableDeposit":true,"coin":"TRX"},{"withdrawMinFee":80.0,"coinType":"ERC20","withdrawMinAmount":160.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"WePower","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"WPR"},{"withdrawMinFee":0.01,"coinType":"stellar","withdrawMinAmount":21.0,"withdrawRemark":"","orgAddress":"GBJNV2MQA7M5GNBRDFW46JLXIN7ZLYVVM4UW4CWDZO4KZKXIXCRYHMH2","txUrl":"https://stellar.expert/explorer/public/tx/{txId}","userAddressName":"MEMO","withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Stellar","tradePrecision":4,"depositRemark":"Please specify memo for deposit, or the transfer will fail to be credited. | 充值时请务必指定memo，否则将无法入账。\r\n","enableWithdraw":true,"enableDeposit":true,"coin":"XLM"},{"withdrawMinFee":2.0,"coinType":"ERC20","withdrawMinAmount":4.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"0X","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"ZRX"},{"withdrawMinFee":800.0,"coinType":"ERC20","withdrawMinAmount":6000.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Decentralized Accessible Content Chain","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"DACC"},{"withdrawMinFee":70.0,"coinType":"ERC20","withdrawMinAmount":500.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"DATx","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"DATX"},{"withdrawMinFee":700.0,"coinType":"ERC20","withdrawMinAmount":1500.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Dent","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"DENT"},{"withdrawMinFee":32.0,"coinType":"ERC20","withdrawMinAmount":210.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Electrify.Asia","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"ELEC"},{"withdrawMinFee":5.0,"coinType":null,"withdrawMinAmount":10.0,"withdrawRemark":null,"orgAddress":null,"txUrl":null,"userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"GALA","tradePrecision":4,"depositRemark":null,"enableWithdraw":false,"enableDeposit":false,"coin":"GALA"},{"withdrawMinFee":100.0,"coinType":"ERC20","withdrawMinAmount":1000.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"IOStoken","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"IOST"},{"withdrawMinFee":150.0,"coinType":"ERC20","withdrawMinAmount":300.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"IoTeX","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"IOTX"},{"withdrawMinFee":50.0,"coinType":"ERC20","withdrawMinAmount":300.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"LALA World","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"LALA"},{"withdrawMinFee":10.0,"coinType":"ERC20","withdrawMinAmount":100.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Loom Network","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"LOOM"},{"withdrawMinFee":15.0,"coinType":"ERC20","withdrawMinAmount":100.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Decentraland","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"MANA"},{"withdrawMinFee":2.0,"coinType":"ERC20","withdrawMinAmount":15.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"nUSD","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"NUSD"},{"withdrawMinFee":15.0,"coinType":"ERC20","withdrawMinAmount":500.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"OPEN Platform","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"OPEN"},{"withdrawMinFee":0.5,"coinType":null,"withdrawMinAmount":1.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://chainz.cryptoid.info/pura/tx.dws?{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Pura","tradePrecision":6,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"PURA"},{"withdrawMinFee":4.0,"coinType":"NEP","withdrawMinAmount":25.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"http://state.otcgo.cn/traninfo.html?id=0x{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Phantasma","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"SOUL"},{"withdrawMinFee":1.0,"coinType":"ERC20","withdrawMinAmount":10.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"TomoChain","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"TOMO"},{"withdrawMinFee":14.0,"coinType":"ERC20","withdrawMinAmount":90.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"OriginTrail","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"TRAC"},{"withdrawMinFee":30.0,"coinType":"ERC20","withdrawMinAmount":200.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Zinc","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"ZINC"},{"withdrawMinFee":1.0,"coinType":"SPHTX","withdrawMinAmount":55.0,"withdrawRemark":null,"orgAddress":"kucoin","txUrl":"https://explorer.sophiatx.com/transactions/{txId}","userAddressName":"memo","withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"SophiaTX","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"SPHTX"},{"withdrawMinFee":30.0,"coinType":"ERC20","withdrawMinAmount":100.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"carVertical","tradePrecision":2,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"CV"},{"withdrawMinFee":10.0,"coinType":"ERC20","withdrawMinAmount":20.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Adbank","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"ADB"},{"withdrawMinFee":10.0,"coinType":"ERC20","withdrawMinAmount":200.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Block Array","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"ARY"},{"withdrawMinFee":1000.0,"coinType":"ERC20","withdrawMinAmount":8000.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"BABB","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"BAX"},{"withdrawMinFee":1.0,"coinType":null,"withdrawMinAmount":5.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://bosradar.com/transaction/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"BOScoin","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"BOS"},{"withdrawMinFee":2.0,"coinType":"ERC20","withdrawMinAmount":1.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Centra","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":false,"coin":"CTR"},{"withdrawMinFee":7.0,"coinType":"ERC20","withdrawMinAmount":80.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Debitum Network","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"DEB"},{"withdrawMinFee":20.0,"coinType":"ERC20","withdrawMinAmount":130.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Endor Protocol","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"EDR"},{"withdrawMinFee":4.0,"coinType":"ERC20","withdrawMinAmount":10.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"aelf","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"ELF"},{"withdrawMinFee":4.0,"coinType":"ERC20","withdrawMinAmount":50.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Gladius Token","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"GLA"},{"withdrawMinFee":0.5,"coinType":"ERC20","withdrawMinAmount":1.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Hawala.Today","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":false,"coin":"HAT"},{"withdrawMinFee":3.0,"coinType":"ERC20","withdrawMinAmount":10.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"IUNGO","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"ING"},{"withdrawMinFee":1.0,"coinType":"ERC20","withdrawMinAmount":50.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"IoT Chain","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"ITC"},{"withdrawMinFee":200.0,"coinType":"ERC20","withdrawMinAmount":1000.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"SelfKey","tradePrecision":4,"depositRemark":"Please be aware that this address is only for depositing SelfKey (KEY), depositing other assets into this address will cause fund loss and it's not revertible.","enableWithdraw":true,"enableDeposit":true,"coin":"KEY"},{"withdrawMinFee":2.0,"coinType":"ERC20","withdrawMinAmount":10.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Matrix AI Network","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"MAN"},{"withdrawMinFee":10.0,"coinType":"ERC20","withdrawMinAmount":20.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Medicalchain","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"MTN"},{"withdrawMinFee":100.0,"coinType":"ERC20","withdrawMinAmount":800.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Merculet","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"MVP"},{"withdrawMinFee":100.0,"coinType":"ERC20","withdrawMinAmount":500.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Odyssey","tradePrecision":2,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"OCN"},{"withdrawMinFee":10.0,"coinType":"ERC20","withdrawMinAmount":1.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Po.et","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":false,"coin":"POE"},{"withdrawMinFee":10.0,"coinType":"ERC20","withdrawMinAmount":100.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"SunContract","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"SNC"},{"withdrawMinFee":5.0,"coinType":"ERC20","withdrawMinAmount":10.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Trade Token","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"TIO"},{"withdrawMinFee":10.0,"coinType":"ERC20","withdrawMinAmount":100.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"UTRUST","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"UTK"},{"withdrawMinFee":0.1,"coinType":null,"withdrawMinAmount":1.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://solaris.blockexplorer.pro/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Solaris","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"XLR"},{"withdrawMinFee":50.0,"coinType":"ERC20","withdrawMinAmount":300.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Zilliqa","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"ZIL"},{"withdrawMinFee":20.0,"coinType":"ERC20","withdrawMinAmount":200.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Cappasity","tradePrecision":2,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"CAPP"},{"withdrawMinFee":70.0,"coinType":"ERC20","withdrawMinAmount":500.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"SwissBorg","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"CHSB"},{"withdrawMinFee":5.0,"coinType":"ERC20","withdrawMinAmount":10.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"CoinFi","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"COFI"},{"withdrawMinFee":6.0,"coinType":"ERC20","withdrawMinAmount":50.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"DADI","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"DADI"},{"withdrawMinFee":0.002,"coinType":null,"withdrawMinAmount":0.05,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://chainz.cryptoid.info/dash/tx.dws?{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Dash","tradePrecision":8,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"DASH"},{"withdrawMinFee":3.0,"coinType":"ERC20","withdrawMinAmount":10.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"eBitcoin","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"EBTC"},{"withdrawMinFee":4.0,"coinType":"ERC20","withdrawMinAmount":10.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"LOCIcoin","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"LOCI"},{"withdrawMinFee":30.0,"coinType":"MOBI","withdrawMinAmount":200.0,"withdrawRemark":"","orgAddress":"GBJNV2MQA7M5GNBRDFW46JLXIN7ZLYVVM4UW4CWDZO4KZKXIXCRYHMH2","txUrl":"https://stellar.expert/explorer/public/tx/{txid}","userAddressName":"MEMO","withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Mobius","tradePrecision":4,"depositRemark":"Please specify memo for deposit, or the transfer will fail to be credited. | 充值时请务必指定memo，否则将无法入账。","enableWithdraw":true,"enableDeposit":true,"coin":"MOBI"},{"withdrawMinFee":5.0,"coinType":"ERC20","withdrawMinAmount":100.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Blockport","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"BPT"},{"withdrawMinFee":3.0,"coinType":"ERC20","withdrawMinAmount":10.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Covesting","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"COV"},{"withdrawMinFee":140.0,"coinType":"ERC20","withdrawMinAmount":1000.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Gatcoin","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"GAT"},{"withdrawMinFee":0.5,"coinType":"ERC20","withdrawMinAmount":10.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Hacken","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"HKN"},{"withdrawMinFee":1.0,"coinType":"ERC20","withdrawMinAmount":10.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Oyster Pearl","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"PRL"},{"withdrawMinFee":300.0,"coinType":"ERC20","withdrawMinAmount":2000.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"SpherePay","tradePrecision":2,"depositRemark":null,"enableWithdraw":false,"enableDeposit":false,"coin":"SAY"},{"withdrawMinFee":1.0,"coinType":"ERC20","withdrawMinAmount":10.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"TrueFlip","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"TFL"},{"withdrawMinFee":8.0,"coinType":"ERC20","withdrawMinAmount":100.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"WAX","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"WAX"},{"withdrawMinFee":3.5,"coinType":"ERC20","withdrawMinAmount":8.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Aion","tradePrecision":6,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"AION"},{"withdrawMinFee":35.0,"coinType":"ERC20","withdrawMinAmount":150.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"KickCoin","tradePrecision":4,"depositRemark":"Please be aware that this wallet address does not support to receive KICK bonus.","enableWithdraw":true,"enableDeposit":true,"coin":"KICK"},{"withdrawMinFee":20.0,"coinType":"ERC20","withdrawMinAmount":100.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Restart Energy","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"MWAT"},{"withdrawMinFee":40.0,"coinType":"ERC20","withdrawMinAmount":300.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"HEROcoin","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"PLAY"},{"withdrawMinFee":40.0,"coinType":"ERC20","withdrawMinAmount":100.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Pareto Network","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"PARETO"},{"withdrawMinFee":2.0,"coinType":"ERC20","withdrawMinAmount":10.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Revain","tradePrecision":6,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"R"},{"withdrawMinFee":5.0,"coinType":"ERC20","withdrawMinAmount":10.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"LAToken","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"LA"},{"withdrawMinFee":20.0,"coinType":"ERC20","withdrawMinAmount":150.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}\r\n","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"AURORA","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"AOA"},{"withdrawMinFee":5.0E-4,"coinType":null,"withdrawMinAmount":0.01,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://blockdozer.com/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":6,"infoUrl":null,"enable":true,"name":"Bitcoin Cash","tradePrecision":8,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"BCH"},{"withdrawMinFee":2.0,"coinType":"ERC20","withdrawMinAmount":5.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Change","tradePrecision":6,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"CAG"},{"withdrawMinFee":0.005,"coinType":null,"withdrawMinAmount":0.01,"withdrawRemark":null,"orgAddress":null,"txUrl":null,"userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Bitcoin God","tradePrecision":8,"depositRemark":null,"enableWithdraw":false,"enableDeposit":false,"coin":"GOD"},{"withdrawMinFee":30.0,"coinType":"ERC20","withdrawMinAmount":100.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"CoinMeet","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":false,"coin":"MEE"},{"withdrawMinFee":2.0,"coinType":"ERC20","withdrawMinAmount":4.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Modum","tradePrecision":6,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"MOD"},{"withdrawMinFee":5.0,"coinType":"ERC20","withdrawMinAmount":10.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Publica","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"PBL"},{"withdrawMinFee":0.3,"coinType":"ERC20","withdrawMinAmount":0.5,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Populous","tradePrecision":6,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"PPT"},{"withdrawMinFee":45.0,"coinType":"ERC20","withdrawMinAmount":100.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Quantstamp","tradePrecision":6,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"QSP"},{"withdrawMinFee":30.0,"coinType":"ERC20","withdrawMinAmount":60.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"SONM","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"SNM"},{"withdrawMinFee":10.0,"coinType":"ERC20","withdrawMinAmount":100.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"SportyCo","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"SPF"},{"withdrawMinFee":0.5,"coinType":null,"withdrawMinAmount":10.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"http://aschd.org/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Asch","tradePrecision":6,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"XAS"},{"withdrawMinFee":50.0,"coinType":"ERC20","withdrawMinAmount":100.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Bounty0x","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"BNTY"},{"withdrawMinFee":0.01,"coinType":null,"withdrawMinAmount":1.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://explorer.btcprivate.org/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Bitcoin Private","tradePrecision":6,"depositRemark":null,"enableWithdraw":false,"enableDeposit":false,"coin":"BTCP"},{"withdrawMinFee":1.0,"coinType":"ERC20","withdrawMinAmount":10.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Dragonchain","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"DRGN"},{"withdrawMinFee":10.0,"coinType":"ERC20","withdrawMinAmount":20.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"CoinMeet","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":false,"coin":"MEET"},{"withdrawMinFee":0.1,"coinType":"NULS","withdrawMinAmount":1.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://nulscan.io/transactionHash?hash={txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Nuls","tradePrecision":6,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"NULS"},{"withdrawMinFee":0.5,"coinType":"ERC20","withdrawMinAmount":5.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"ClearPoll","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"POLL"},{"withdrawMinFee":8.0,"coinType":"ERC20","withdrawMinAmount":15.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Power Ledger","tradePrecision":6,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"POWR"},{"withdrawMinFee":20.0,"coinType":"ERC20","withdrawMinAmount":100.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Snovio","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"SNOV"},{"withdrawMinFee":0.1,"coinType":null,"withdrawMinAmount":1.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://www.blockexperts.com/onion/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"DeepOnion","tradePrecision":4,"depositRemark":null,"enableWithdraw":false,"enableDeposit":false,"coin":"ONION"},{"withdrawMinFee":0.01,"coinType":null,"withdrawMinAmount":0.1,"withdrawRemark":null,"orgAddress":null,"txUrl":"http://gastracker.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Ethereum Classic","tradePrecision":6,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"ETC"},{"withdrawMinFee":10.0,"coinType":"ERC20","withdrawMinAmount":30.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Ambrosus","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"AMB"},{"withdrawMinFee":5.0,"coinType":"ERC20","withdrawMinAmount":1.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Stox","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":false,"coin":"STX"},{"withdrawMinFee":3.0,"coinType":"ERC20","withdrawMinAmount":5.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Elixir","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"ELIX"},{"withdrawMinFee":20.0,"coinType":"ERC20","withdrawMinAmount":100.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"STK","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"STK"},{"withdrawMinFee":10.0,"coinType":"ERC20","withdrawMinAmount":20.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Flixxo","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"FLIXX"},{"withdrawMinFee":0.3,"coinType":"ERC20","withdrawMinAmount":1.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Genesis Vision","tradePrecision":6,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"GVT"},{"withdrawMinFee":0.005,"coinType":null,"withdrawMinAmount":0.01,"withdrawRemark":null,"orgAddress":null,"txUrl":"http://explorer.btcd.io/#/TX?TX={txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":6,"infoUrl":null,"enable":true,"name":"Bitcoin Diamond","tradePrecision":8,"depositRemark":null,"enableWithdraw":false,"enableDeposit":false,"coin":"BCD"},{"withdrawMinFee":0.5,"coinType":"ERC20","withdrawMinAmount":1.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Confido","tradePrecision":6,"depositRemark":null,"enableWithdraw":false,"enableDeposit":false,"coin":"CFD"},{"withdrawMinFee":0.0,"coinType":"GAS","withdrawMinAmount":0.2,"withdrawRemark":null,"orgAddress":null,"txUrl":"http://state.otcgo.cn/traninfo.html?id=0x{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":6,"infoUrl":null,"enable":true,"name":"NeoGas","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"GAS"},{"withdrawMinFee":2.0,"coinType":"ERC20","withdrawMinAmount":10.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Horizon State","tradePrecision":6,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"HST"},{"withdrawMinFee":5.0,"coinType":"ERC20","withdrawMinAmount":10.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Raiden Network","tradePrecision":6,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"RDN"},{"withdrawMinFee":4.0,"coinType":"ERC20","withdrawMinAmount":25.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"SHL","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"SHL"},{"withdrawMinFee":12.0,"coinType":"ERC20","withdrawMinAmount":25.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Substratum","tradePrecision":2,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"SUB"},{"withdrawMinFee":5.0,"coinType":"ERC20","withdrawMinAmount":10.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Unikoin Gold","tradePrecision":6,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"UKG"},{"withdrawMinFee":0.5,"coinType":"ERC20","withdrawMinAmount":1.5,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Walton","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"WTC"},{"withdrawMinFee":0.1,"coinType":null,"withdrawMinAmount":5.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"http://explorer.nebl.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Neblio","tradePrecision":6,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"NEBL"},{"withdrawMinFee":10.0,"coinType":"ERC20","withdrawMinAmount":20.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Polymath","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"POLY"},{"withdrawMinFee":2.0,"coinType":"ERC20","withdrawMinAmount":5.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"RChain","tradePrecision":6,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"RHOC"},{"withdrawMinFee":3.2,"coinType":null,"withdrawMinAmount":20.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"http://omniexplorer.info/lookuptx.aspx?txid={txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":3,"infoUrl":null,"enable":true,"name":"USDT","tradePrecision":6,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"USDT"},{"withdrawMinFee":0.005,"coinType":null,"withdrawMinAmount":0.01,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://btgexp.com/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Bitcoin Gold","tradePrecision":8,"depositRemark":null,"enableWithdraw":false,"enableDeposit":false,"coin":"BTG"},{"withdrawMinFee":0.5,"coinType":null,"withdrawMinAmount":1.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://digiexplorer.info/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Digibyte","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"DGB"},{"withdrawMinFee":0.5,"coinType":"ERC20","withdrawMinAmount":1.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Everex","tradePrecision":6,"depositRemark":null,"enableWithdraw":true,"enableDeposit":false,"coin":"EVX"},{"withdrawMinFee":0.01,"coinType":null,"withdrawMinAmount":0.1,"withdrawRemark":null,"orgAddress":null,"txUrl":"http://hshare-explorer.h.cash/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":6,"infoUrl":null,"enable":true,"name":"Hshare","tradePrecision":4,"depositRemark":null,"enableWithdraw":false,"enableDeposit":false,"coin":"HSR"},{"withdrawMinFee":3.5,"coinType":"ERC20","withdrawMinAmount":8.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Kyber Network","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"KNC"},{"withdrawMinFee":0.001,"coinType":null,"withdrawMinAmount":0.1,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://live.blockcypher.com/ltc/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":6,"infoUrl":null,"enable":true,"name":"Litecoin","tradePrecision":6,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"LTC"},{"withdrawMinFee":75.0,"coinType":"ERC20","withdrawMinAmount":150.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Monetha","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"MTH"},{"withdrawMinFee":0.4,"coinType":"ERC20","withdrawMinAmount":1.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"OmiseGO","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"OMG"},{"withdrawMinFee":40.0,"coinType":"ERC20","withdrawMinAmount":80.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Request","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"REQ"},{"withdrawMinFee":20.0,"coinType":"ERC20","withdrawMinAmount":40.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"BlockMason","tradePrecision":6,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"BCPT"},{"withdrawMinFee":0.1,"coinType":null,"withdrawMinAmount":0.2,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://explorer.qtum.org/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":6,"infoUrl":null,"enable":true,"name":"Qtum","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"QTUM"},{"withdrawMinFee":5.0,"coinType":"ERC20","withdrawMinAmount":20.0,"withdrawRemark":"","orgAddress":null,"txUrl":"http://blockmeta.com/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Bytom","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"BTM"},{"withdrawMinFee":12.0,"coinType":"ERC20","withdrawMinAmount":30.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Civic","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"CVC"},{"withdrawMinFee":0.01,"coinType":"ERC20","withdrawMinAmount":0.1,"withdrawRemark":null,"orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Ethereum Fog","tradePrecision":6,"depositRemark":null,"enableWithdraw":false,"enableDeposit":false,"coin":"ETF"},{"withdrawMinFee":0.5,"coinType":"ERC20","withdrawMinAmount":1.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"TenXPay","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"PAY"},{"withdrawMinFee":0.05,"coinType":"EOS","withdrawMinAmount":1.0,"withdrawRemark":"","orgAddress":"kucoindoteos","txUrl":"https://eosflare.io/tx/{txId}","userAddressName":"MEMO","withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"EOS","tradePrecision":4,"depositRemark":"Please specify memo for deposit, or the transfer will fail to be credited. | 充值时请务必指定memo，否则将无法入账。","enableWithdraw":true,"enableDeposit":true,"coin":"EOS"},{"withdrawMinFee":20.0,"coinType":"ERC20","withdrawMinAmount":100.0,"withdrawRemark":"","orgAddress":null,"txUrl":"https://etherscan.io/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":12,"infoUrl":null,"enable":true,"name":"Status","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":true,"coin":"SNT"},{"withdrawMinFee":1.0,"coinType":null,"withdrawMinAmount":1.0,"withdrawRemark":null,"orgAddress":null,"txUrl":"http://blackholecoin.cn:3001/tx/{txId}","userAddressName":null,"withdrawFeeRate":0.001,"confirmationCount":6,"infoUrl":null,"enable":true,"name":"Black Hole Coin","tradePrecision":4,"depositRemark":null,"enableWithdraw":true,"enableDeposit":false,"coin":"BHC"}]}`

	json := `{
  "success": true,
  "code": "OK",
  "msg": "Operation succeeded.",
  "data": {
    "orderOid": "596186ad07015679730ffa02"
  }
}`
	endpoint := "http://localhost:4243"
	rt := &FakeRoundTripper{message: jsonPrecision, status: http.StatusOK}
	precisionMap := make(map[string]map[string]models.Precisions)
	settlementMap := make(map[string]models.Precisions)
	settlementMap["BTC"] = models.Precisions{AmountPrecision:4,PricePrecision:8}
	precisionMap["ETH"] = settlementMap
	client :=&KucoinApi{
		BaseURL:           endpoint,
		RateCacheDuration: 30 * time.Second,
		HttpClient:        &http.Client{Transport: rt},
		settlements:       []string{"BTC"},
		precisionMap:      precisionMap,
		rateMap:           nil,
		volumeMap:         nil,
		rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		m:                 new(sync.Mutex),
	}
	rt.message = json
	orderId, err := client.Order("ETH", "BTC", models.Bid, 1000000, 0.01)
	if err != nil {
		t.Error(err)
	}
	if orderId != "596186ad07015679730ffa02" {
		t.Errorf("KucoinPrivateApi: Expected %v. Got %v", "596186ad07015679730ffa02", orderId)
	}
	rt.message = json
	err = client.CancelOrder("ETH", "BTC", models.Bid, orderId)
	if err != nil {
		t.Error(err)
	}
}
