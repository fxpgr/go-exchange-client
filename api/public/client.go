package public

import (
	"errors"
	"github.com/fxpgr/go-ccex-api-client/models"
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
	case "bitflyer":
		return NewBitflyerPublicApi()
	case "poloniex":
		return NewPoloniexPublicApi()
	case "hitbtc":
		return NewHitbtcPublicApi()
	}
	return nil, errors.New("failed to init exchange api")
}
