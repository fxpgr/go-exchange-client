package public

import (
	"errors"
	"github.com/fxpgr/go-exchange-client/models"
	"net/http"
	"strings"
	"sync"
)

var (
	clientMap map[string]PublicClient
	mtx       sync.Mutex
)

//go:generate mockery -name=PublicClient
type PublicClient interface {
	Volume(trading string, settlement string) (float64, error)
	CurrencyPairs() ([]models.CurrencyPair, error)
	Rate(trading string, settlement string) (float64, error)
	RateMap() (map[string]map[string]float64, error)
	VolumeMap() (map[string]map[string]float64, error)
	OrderBookTickMap() (map[string]map[string]models.OrderBookTick, error)
	FrozenCurrency() ([]string, error)
	Board(trading string, settlement string) (*models.Board, error)
	Precise(trading string, settlement string) (*models.Precisions, error)

	SetTransport(transport http.RoundTripper) error
}

func NewDefaultClient(exchangeName string) PublicClient {
	mtx.Lock()
	defer mtx.Unlock()
	if clientMap[strings.ToLower(exchangeName)] == nil {
		cli, err := NewClient(exchangeName)
		if err != nil {
			panic(err)
		}
		clientMap[strings.ToLower(exchangeName)] = cli
		return cli
	}
	return clientMap[strings.ToLower(exchangeName)]
}

func NewClient(exchangeName string) (PublicClient, error) {
	switch strings.ToLower(exchangeName) {
	case "binance":
		return NewBinancePublicApi()
	case "bitflyer":
		return NewBitflyerPublicApi()
	case "poloniex":
		return NewPoloniexPublicApi()
	case "hitbtc":
		return NewHitbtcPublicApi()
	case "huobi":
		return NewHuobiPublicApi()
	case "okex":
		return NewOkexPublicApi()
	case "cobinhood":
		return NewCobinhoodPublicApi()
	case "lbank":
		return NewLbankPublicApi()
	case "kucoin":
		return NewKucoinPublicApi()
	case "p2pb2b":
		return NewP2pb2bPublicApi()
	}
	return nil, errors.New("failed to init exchange api")
}
