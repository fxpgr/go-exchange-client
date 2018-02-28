package public

import (
	"errors"
	"github.com/fxpgr/go-ccex-api-client/models"
	"strings"
)

//go:generate mockery -name=PublicClient
type PublicClient interface {
	Volume(trading string, settlement string) (float64, error)
	CurrencyPairs() ([]*models.CurrencyPair, error)
	Rate(trading string, settlement string) (float64, error)
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
