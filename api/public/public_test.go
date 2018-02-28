package public

import (
	"testing"
	"net/http"
	"strings"
	"io/ioutil"
	"math"
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

func newTestBitflyerPublicClient(rt http.RoundTripper) ExchangePublicRepository {
	endpoint := "http://localhost:4243"
	client,err := NewTestBitflyerPublicApi(endpoint,http.Client{Transport: rt})
	if err != nil {
		panic(err)
	}
	return client
}

func newTestBitflyerPrivateClient(rt http.RoundTripper) ExchangePublicRepository {
	endpoint := "http://localhost:4243"
	client,err := NewTestBitflyerPublicApi(endpoint,http.Client{Transport: rt})
	if err != nil {
		panic(err)
	}
	return client
}

func TestBitflyerRate(t *testing.T) {
	t.Parallel()
	jsonBitflyerTicker :=  `{
  "product_code": "BTC_JPY",
  "timestamp": "2015-07-08T02:50:59.97",
  "tick_id": 3579,
  "best_bid": 30000,
  "best_ask": 36640,
  "best_bid_size": 0.1,
  "best_ask_size": 5,
  "total_bid_depth": 15.13,
  "total_ask_depth": 20,
  "ltp": 31690,
  "volume": 16819.26,
  "volume_by_product": 6819.26
}`
	client:=newTestBitflyerPublicClient(&FakeRoundTripper{message:jsonBitflyerTicker, status:http.StatusOK})
	rate,err := client.Rate("BTC","JPY")
	if err != nil {
		panic(err)
	}
	if rate !=math.Trunc(31690) {
		t.Errorf("BitflyerPublicApi: Expected %v. Got %v",31690,rate)
	}
}

func TestBitflyerVolume(t *testing.T) {
	t.Parallel()
	jsonBitflyerTicker :=  `{
  "product_code": "BTC_JPY",
  "timestamp": "2015-07-08T02:50:59.97",
  "tick_id": 3579,
  "best_bid": 30000,
  "best_ask": 36640,
  "best_bid_size": 0.1,
  "best_ask_size": 5,
  "total_bid_depth": 15.13,
  "total_ask_depth": 20,
  "ltp": 31690,
  "volume": 16819.26,
  "volume_by_product": 6819.26
}`
	client:=newTestBitflyerPublicClient(&FakeRoundTripper{message:jsonBitflyerTicker, status:http.StatusOK})
	volume,err := client.Volume("BTC","JPY")
	if err != nil {
		panic(err)
	}
	if volume !=16819.26 {
		t.Errorf("BitflyerPublicApi: Expected %v. Got %v",16819.26,volume)
	}
}

func TestBitflyerCurrencyPairs(t *testing.T) {
	t.Parallel()
	jsonBitflyerTicker :=  `{
  "product_code": "BTC_JPY",
  "timestamp": "2015-07-08T02:50:59.97",
  "tick_id": 3579,
  "best_bid": 30000,
  "best_ask": 36640,
  "best_bid_size": 0.1,
  "best_ask_size": 5,
  "total_bid_depth": 15.13,
  "total_ask_depth": 20,
  "ltp": 31690,
  "volume": 16819.26,
  "volume_by_product": 6819.26
}`
	client:=newTestBitflyerPublicClient(&FakeRoundTripper{message:jsonBitflyerTicker, status:http.StatusOK})
	pairs,err := client.CurrencyPairs()
	if err != nil {
		panic(err)
	}
	for _,_ = range pairs {
	}
}
